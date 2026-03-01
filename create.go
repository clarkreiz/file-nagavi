package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const DefaultPieceLength = 256 * 1024 // 256 KiB

func CreateTorrent(filePath string, pieceLength int64) (*Torrent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	fileSize := stat.Size()

	pieces, err := computePieces(f, pieceLength)
	if err != nil {
		return nil, fmt.Errorf("compute pieces: %w", err)
	}

	info := Info{
		Name:        filepath.Base(filePath),
		PieceLength: pieceLength,
		Pieces:      pieces,
		Length:      fileSize,
	}

	infoDict := map[string]any{
		"length":       fileSize,
		"name":         info.Name,
		"piece length": pieceLength,
		"pieces":       piecesToString(pieces),
	}

	infoEncoded, err := Encode(infoDict)
	if err != nil {
		return nil, fmt.Errorf("encode info: %w", err)
	}

	return &Torrent{
		Info:     info,
		InfoHash: sha1.Sum([]byte(infoEncoded)),
	}, nil
}

func WriteTorrentFile(tor *Torrent, destPath string) error {
	infoDict := map[string]any{
		"length":       tor.Info.Length,
		"name":         tor.Info.Name,
		"piece length": tor.Info.PieceLength,
		"pieces":       piecesToString(tor.Info.Pieces),
	}
	dict := map[string]any{"info": infoDict}
	if tor.Announce != "" {
		dict["announce"] = tor.Announce
	}

	encoded, err := Encode(dict)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return os.WriteFile(destPath, []byte(encoded), 0644)
}

func computePieces(r io.Reader, pieceLength int64) ([][20]byte, error) {
	var pieces [][20]byte
	buf := make([]byte, pieceLength)

	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			pieces = append(pieces, sha1.Sum(buf[:n]))
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return pieces, nil
}

func piecesToString(pieces [][20]byte) string {
	b := make([]byte, len(pieces)*20)
	for i, p := range pieces {
		copy(b[i*20:], p[:])
	}
	return string(b)
}
