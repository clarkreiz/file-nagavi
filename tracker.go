package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerID     [20]byte
	Port       uint16
	Uploaded   int64
	Downloaded int64
	Left       int64
	Event      string // "started" | "stopped" | "completed" | ""
}

type Peer struct {
	IP   net.IP
	Port uint16
}

func (p Peer) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

type AnnounceResponse struct {
	Interval int64
	Peers    []Peer
}

func NewPeerID() [20]byte {
	var id [20]byte
	copy(id[:8], "-GT0001-")
	rand.Read(id[8:])
	return id
}

func Announce(trackerURL string, req AnnounceRequest) (*AnnounceResponse, error) {
	u := trackerURL + "?" + buildQuery(req)

	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseAnnounceResponse(body)
}

func buildQuery(req AnnounceRequest) string {
	var b strings.Builder
	b.WriteString("info_hash=")
	b.WriteString(percentEncode(req.InfoHash[:]))
	b.WriteString("&peer_id=")
	b.WriteString(percentEncode(req.PeerID[:]))
	fmt.Fprintf(&b, "&port=%d&uploaded=%d&downloaded=%d&left=%d&compact=1",
		req.Port, req.Uploaded, req.Downloaded, req.Left)
	if req.Event != "" {
		b.WriteString("&event=")
		b.WriteString(req.Event)
	}
	return b.String()
}

// percentEncode encodes binary data per RFC 3986.
func percentEncode(b []byte) string {
	var buf strings.Builder
	for _, c := range b {
		if isUnreserved(c) {
			buf.WriteByte(c)
		} else {
			fmt.Fprintf(&buf, "%%%02X", c)
		}
	}
	return buf.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func parseAnnounceResponse(data []byte) (*AnnounceResponse, error) {
	raw, err := Decode(string(data))
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	dict, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response is not a dict")
	}
	if reason, ok := dict["failure reason"].(string); ok {
		return nil, fmt.Errorf("tracker: %s", reason)
	}

	resp := &AnnounceResponse{}

	if v, ok := dict["interval"].(int64); ok {
		resp.Interval = v
	}

	switch peers := dict["peers"].(type) {
	case string:
		// compact format: 6 bytes per peer (4 IP + 2 port)
		parsed, err := parseCompactPeers([]byte(peers))
		if err != nil {
			return nil, fmt.Errorf("parse compact peers: %w", err)
		}
		resp.Peers = parsed
	case []any:
		// non-compact format: list of dicts
		for _, p := range peers {
			peer, err := parsePeerDict(p)
			if err != nil {
				return nil, fmt.Errorf("parse peer: %w", err)
			}
			resp.Peers = append(resp.Peers, *peer)
		}
	}

	return resp, nil
}

func parseCompactPeers(data []byte) ([]Peer, error) {
	if len(data)%6 != 0 {
		return nil, fmt.Errorf("length %d is not a multiple of 6", len(data))
	}
	peers := make([]Peer, len(data)/6)
	for i := range peers {
		off := i * 6
		peers[i] = Peer{
			IP:   net.IP(data[off : off+4]),
			Port: binary.BigEndian.Uint16(data[off+4 : off+6]),
		}
	}
	return peers, nil
}

func parsePeerDict(raw any) (*Peer, error) {
	d, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("peer is not a dict")
	}
	ip, ok := d["ip"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'ip'")
	}
	port, ok := d["port"].(int64)
	if !ok {
		return nil, fmt.Errorf("missing 'port'")
	}
	return &Peer{IP: net.ParseIP(ip), Port: uint16(port)}, nil
}
