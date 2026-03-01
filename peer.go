package main

import (
	"fmt"
	"io"
	"net"
	"time"
)

const protocol = "BitTorrent protocol"

var reserved = [8]byte{}

type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

func (h *Handshake) encode() []byte {
	buf := make([]byte, 1+len(protocol)+8+20+20)
	i := 0
	buf[i] = byte(len(protocol))
	i++
	i += copy(buf[i:], protocol)
	i += copy(buf[i:], reserved[:])
	i += copy(buf[i:], h.InfoHash[:])
	copy(buf[i:], h.PeerID[:])
	return buf
}

func decodeHandshake(r io.Reader) (*Handshake, error) {
	// read protocol length
	var pstrLen [1]byte
	if _, err := io.ReadFull(r, pstrLen[:]); err != nil {
		return nil, fmt.Errorf("read pstrlen: %w", err)
	}
	if pstrLen[0] != byte(len(protocol)) {
		return nil, fmt.Errorf("unexpected protocol length: %d", pstrLen[0])
	}

	// read protocol string
	pstr := make([]byte, pstrLen[0])
	if _, err := io.ReadFull(r, pstr); err != nil {
		return nil, fmt.Errorf("read pstr: %w", err)
	}
	if string(pstr) != protocol {
		return nil, fmt.Errorf("unexpected protocol: %q", pstr)
	}

	// reserved (8 bytes) — ignore
	var res [8]byte
	if _, err := io.ReadFull(r, res[:]); err != nil {
		return nil, fmt.Errorf("read reserved: %w", err)
	}

	var h Handshake
	if _, err := io.ReadFull(r, h.InfoHash[:]); err != nil {
		return nil, fmt.Errorf("read info_hash: %w", err)
	}
	if _, err := io.ReadFull(r, h.PeerID[:]); err != nil {
		return nil, fmt.Errorf("read peer_id: %w", err)
	}
	return &h, nil
}

type PeerConn struct {
	conn     net.Conn
	InfoHash [20]byte
	PeerID   [20]byte
}

func Connect(peer Peer, infoHash [20]byte, peerID [20]byte) (*PeerConn, error) {
	addr := net.JoinHostPort(peer.IP.String(), fmt.Sprintf("%d", peer.Port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	hs := &Handshake{InfoHash: infoHash, PeerID: peerID}
	if _, err := conn.Write(hs.encode()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send handshake: %w", err)
	}

	remote, err := decodeHandshake(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("recv handshake: %w", err)
	}
	if remote.InfoHash != infoHash {
		conn.Close()
		return nil, fmt.Errorf("info_hash mismatch")
	}

	conn.SetDeadline(time.Time{}) // сбросить дедлайн

	return &PeerConn{
		conn:     conn,
		InfoHash: remote.InfoHash,
		PeerID:   remote.PeerID,
	}, nil
}

func (c *PeerConn) Close() error {
	return c.conn.Close()
}
