package main

import (
	"crypto/sha1"
	"fmt"
	"os"
)

type FileInfo struct {
	Length int64
	Path   []string
}

type Info struct {
	Name        string
	PieceLength int64
	Pieces      [][20]byte
	Length      int64
	Files       []FileInfo
}

func (i *Info) IsMultiFile() bool {
	return len(i.Files) > 0
}

type PieceInfo struct {
	Index  int
	Hash   [20]byte
	Offset int64
	Size   int64
}

func (i *Info) PieceList() []PieceInfo {
	total := i.TotalLength()
	list := make([]PieceInfo, len(i.Pieces))
	for idx, h := range i.Pieces {
		offset := int64(idx) * i.PieceLength
		size := i.PieceLength
		if offset+size > total {
			size = total - offset
		}
		list[idx] = PieceInfo{Index: idx, Hash: h, Offset: offset, Size: size}
	}
	return list
}

func (i *Info) TotalLength() int64 {
	if !i.IsMultiFile() {
		return i.Length
	}
	var total int64
	for _, f := range i.Files {
		total += f.Length
	}
	return total
}

type Torrent struct {
	Announce     string
	AnnounceList [][]string
	Info         Info
	InfoHash     [20]byte
	Comment      string
	CreatedBy    string
	CreationDate int64
	Encoding     string
}

func ParseTorrent(path string) (*Torrent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return parseTorrent(data)
}

func parseTorrent(data []byte) (*Torrent, error) {
	raw, err := Decode(string(data))
	if err != nil {
		return nil, fmt.Errorf("bencode decode: %w", err)
	}
	dict, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("torrent must be a bencode dict")
	}

	t := &Torrent{}

	if v, ok := dict["announce"].(string); ok {
		t.Announce = v
	}
	if v, ok := dict["announce-list"].([]any); ok {
		t.AnnounceList = parseAnnounceList(v)
	}
	if v, ok := dict["comment"].(string); ok {
		t.Comment = v
	}
	if v, ok := dict["created by"].(string); ok {
		t.CreatedBy = v
	}
	if v, ok := dict["creation date"].(int64); ok {
		t.CreationDate = v
	}
	if v, ok := dict["encoding"].(string); ok {
		t.Encoding = v
	}

	infoRaw, ok := dict["info"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'info' dict")
	}

	info, err := parseInfo(infoRaw)
	if err != nil {
		return nil, fmt.Errorf("parse info: %w", err)
	}
	t.Info = *info

	infoBytes, err := extractInfoBytes(data)
	if err != nil {
		return nil, fmt.Errorf("extract info bytes: %w", err)
	}
	t.InfoHash = sha1.Sum(infoBytes)

	return t, nil
}

func parseAnnounceList(raw []any) [][]string {
	var result [][]string
	for _, tier := range raw {
		tierList, ok := tier.([]any)
		if !ok {
			continue
		}
		var urls []string
		for _, u := range tierList {
			if s, ok := u.(string); ok {
				urls = append(urls, s)
			}
		}
		result = append(result, urls)
	}
	return result
}

func parseInfo(info map[string]any) (*Info, error) {
	result := &Info{}

	name, ok := info["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'name'")
	}
	result.Name = name

	pieceLength, ok := info["piece length"].(int64)
	if !ok {
		return nil, fmt.Errorf("missing 'piece length'")
	}
	result.PieceLength = pieceLength

	piecesRaw, ok := info["pieces"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'pieces'")
	}
	if len(piecesRaw)%20 != 0 {
		return nil, fmt.Errorf("'pieces' length %d is not a multiple of 20", len(piecesRaw))
	}
	for i := 0; i < len(piecesRaw); i += 20 {
		var h [20]byte
		copy(h[:], piecesRaw[i:i+20])
		result.Pieces = append(result.Pieces, h)
	}

	switch {
	case info["length"] != nil:
		length, ok := info["length"].(int64)
		if !ok {
			return nil, fmt.Errorf("'length' must be an integer")
		}
		result.Length = length

	case info["files"] != nil:
		files, ok := info["files"].([]any)
		if !ok {
			return nil, fmt.Errorf("'files' must be a list")
		}
		for i, f := range files {
			fDict, ok := f.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("files[%d] is not a dict", i)
			}
			fi, err := parseFileInfo(fDict)
			if err != nil {
				return nil, fmt.Errorf("files[%d]: %w", i, err)
			}
			result.Files = append(result.Files, *fi)
		}

	default:
		return nil, fmt.Errorf("info must contain 'length' (single-file) or 'files' (multi-file)")
	}

	return result, nil
}

func parseFileInfo(f map[string]any) (*FileInfo, error) {
	length, ok := f["length"].(int64)
	if !ok {
		return nil, fmt.Errorf("missing 'length'")
	}

	pathRaw, ok := f["path"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing 'path'")
	}
	path := make([]string, 0, len(pathRaw))
	for i, p := range pathRaw {
		s, ok := p.(string)
		if !ok {
			return nil, fmt.Errorf("path[%d] is not a string", i)
		}
		path = append(path, s)
	}

	return &FileInfo{Length: length, Path: path}, nil
}

// extractInfoBytes returns the raw bencoded bytes of the "info" value
// from the top-level dict, used to compute InfoHash correctly.
func extractInfoBytes(data []byte) ([]byte, error) {
	s := string(data)
	if len(s) == 0 || s[0] != 'd' {
		return nil, fmt.Errorf("expected top-level dict")
	}
	pos := 1
	for pos < len(s) && s[pos] != 'e' {
		key, next, err := decodeString(s, pos)
		if err != nil {
			return nil, fmt.Errorf("decode key: %w", err)
		}
		pos = next

		if key == "info" {
			start := pos
			_, end, err := decode(s, pos)
			if err != nil {
				return nil, fmt.Errorf("decode info value: %w", err)
			}
			return data[start:end], nil
		}

		_, next, err = decode(s, pos)
		if err != nil {
			return nil, fmt.Errorf("skip value: %w", err)
		}
		pos = next
	}
	return nil, fmt.Errorf("'info' key not found in torrent")
}
