package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

type MessageID uint8

const (
	MsgChoke         MessageID = 0
	MsgUnchoke       MessageID = 1
	MsgInterested    MessageID = 2
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4
	MsgBitfield      MessageID = 5
	MsgRequest       MessageID = 6
	MsgPiece         MessageID = 7
	MsgCancel        MessageID = 8
)

type Message struct {
	ID      MessageID
	Payload []byte
}

func ReadMessage(r io.Reader) (*Message, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}
	length := binary.BigEndian.Uint32(lenBuf[:])

	// keep-alive: length == 0, no ID, no payload
	if length == 0 {
		return nil, nil
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	return &Message{ID: MessageID(buf[0]), Payload: buf[1:]}, nil
}

func WriteMessage(w io.Writer, msg *Message) error {
	// keep-alive
	if msg == nil {
		_, err := w.Write([]byte{0, 0, 0, 0})
		return err
	}

	length := uint32(1 + len(msg.Payload))
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[:4], length)
	buf[4] = byte(msg.ID)
	copy(buf[5:], msg.Payload)

	_, err := w.Write(buf)
	return err
}

// --- constructors ---

func NewMsgInterested() *Message {
	return &Message{ID: MsgInterested}
}

func NewMsgNotInterested() *Message {
	return &Message{ID: MsgNotInterested}
}

func NewMsgUnchoke() *Message {
	return &Message{ID: MsgUnchoke}
}

func NewMsgHave(index uint32) *Message {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, index)
	return &Message{ID: MsgHave, Payload: payload}
}

// NewMsgRequest запрашивает блок данных у пира.
// index — номер куска, begin — смещение внутри куска, length — размер блока.
func NewMsgRequest(index, begin, length uint32) *Message {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], begin)
	binary.BigEndian.PutUint32(payload[8:12], length)
	return &Message{ID: MsgRequest, Payload: payload}
}

// --- parsers ---

func ParseHave(msg *Message) (uint32, error) {
	if msg.ID != MsgHave {
		return 0, fmt.Errorf("expected MsgHave, got %d", msg.ID)
	}
	if len(msg.Payload) != 4 {
		return 0, fmt.Errorf("have payload length %d, want 4", len(msg.Payload))
	}
	return binary.BigEndian.Uint32(msg.Payload), nil
}

type PieceBlock struct {
	Index uint32
	Begin uint32
	Data  []byte
}

func ParsePiece(msg *Message) (*PieceBlock, error) {
	if msg.ID != MsgPiece {
		return nil, fmt.Errorf("expected MsgPiece, got %d", msg.ID)
	}
	if len(msg.Payload) < 8 {
		return nil, fmt.Errorf("piece payload too short: %d", len(msg.Payload))
	}
	return &PieceBlock{
		Index: binary.BigEndian.Uint32(msg.Payload[0:4]),
		Begin: binary.BigEndian.Uint32(msg.Payload[4:8]),
		Data:  msg.Payload[8:],
	}, nil
}

func ParseRequest(msg *Message) (index, begin, length uint32, err error) {
	if msg.ID != MsgRequest {
		return 0, 0, 0, fmt.Errorf("expected MsgRequest, got %d", msg.ID)
	}
	if len(msg.Payload) != 12 {
		return 0, 0, 0, fmt.Errorf("request payload length %d, want 12", len(msg.Payload))
	}
	index = binary.BigEndian.Uint32(msg.Payload[0:4])
	begin = binary.BigEndian.Uint32(msg.Payload[4:8])
	length = binary.BigEndian.Uint32(msg.Payload[8:12])
	return
}

// --- PeerConn helpers ---

func (c *PeerConn) ReadMessage() (*Message, error) {
	return ReadMessage(c.conn)
}

func (c *PeerConn) SendMessage(msg *Message) error {
	return WriteMessage(c.conn, msg)
}
