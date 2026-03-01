package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

type Seeder struct {
	tor     *Torrent
	data    []byte
	peerID  [20]byte
}

func NewSeeder(tor *Torrent, filePath string) (*Seeder, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return &Seeder{
		tor:    tor,
		data:   data,
		peerID: NewPeerID(),
	}, nil
}

func (s *Seeder) Listen(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	fmt.Printf("Seeding %q on port %d\n", s.tor.Info.Name, port)
	fmt.Printf("InfoHash: %x\n", s.tor.InfoHash)

	if s.tor.Announce != "" {
		go s.announceLoop(port)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go s.handleConn(conn)
	}
}

func (s *Seeder) announceLoop(port int) {
	req := AnnounceRequest{
		InfoHash: s.tor.InfoHash,
		PeerID:   s.peerID,
		Port:     uint16(port),
		Left:     0,
		Event:    "started",
	}

	interval := 30 * time.Minute

	for {
		resp, err := Announce(s.tor.Announce, req)
		if err != nil {
			fmt.Printf("Tracker announce error: %v\n", err)
		} else {
			fmt.Printf("Tracker: announced OK, %d peers, next in %ds\n", len(resp.Peers), resp.Interval)
			interval = time.Duration(resp.Interval) * time.Second
		}
		req.Event = "" // subsequent announces have no event
		time.Sleep(interval)
	}
}

func (s *Seeder) handleConn(conn net.Conn) {
	defer conn.Close()

	remote, err := decodeHandshake(conn)
	if err != nil {
		return
	}
	if remote.InfoHash != s.tor.InfoHash {
		return
	}

	hs := &Handshake{InfoHash: s.tor.InfoHash, PeerID: s.peerID}
	if _, err := conn.Write(hs.encode()); err != nil {
		return
	}

	// отправляем bitfield — у нас есть все куски
	bitfield := makeBitfield(len(s.tor.Info.Pieces))
	if err := WriteMessage(conn, &Message{ID: MsgBitfield, Payload: bitfield}); err != nil {
		return
	}

	// отправляем unchoke
	if err := WriteMessage(conn, NewMsgUnchoke()); err != nil {
		return
	}

	for {
		msg, err := ReadMessage(conn)
		if err != nil {
			return
		}
		if msg == nil {
			continue
		}
		switch msg.ID {
		case MsgInterested:
			// уже unchoked, ничего не делаем
		case MsgRequest:
			if err := s.serveBlock(conn, msg); err != nil {
				return
			}
		}
	}
}

func (s *Seeder) serveBlock(conn net.Conn, msg *Message) error {
	index, begin, length, err := ParseRequest(msg)
	if err != nil {
		return err
	}

	info := &s.tor.Info
	start := int64(index)*info.PieceLength + int64(begin)
	end := start + int64(length)

	if start < 0 || end > int64(len(s.data)) {
		return fmt.Errorf("block out of bounds")
	}

	block := s.data[start:end]
	payload := make([]byte, 8+len(block))
	payload[0], payload[1], payload[2], payload[3] = byte(index>>24), byte(index>>16), byte(index>>8), byte(index)
	payload[4], payload[5], payload[6], payload[7] = byte(begin>>24), byte(begin>>16), byte(begin>>8), byte(begin)
	copy(payload[8:], block)

	return WriteMessage(conn, &Message{ID: MsgPiece, Payload: payload})
}

// makeBitfield создаёт битовое поле где все биты выставлены (все куски есть).
func makeBitfield(numPieces int) []byte {
	bf := make([]byte, (numPieces+7)/8)
	for i := 0; i < numPieces; i++ {
		bf[i/8] |= 1 << (7 - uint(i%8))
	}
	return bf
}
