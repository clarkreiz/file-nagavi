package main

import (
	"crypto/sha1"
	"fmt"
	"time"
)

const blockSize = 16384 // 16 KiB — стандартный размер блока в BitTorrent

// DownloadPiece скачивает один кусок от пира и проверяет его SHA1.
// Включает в себя отправку interested и ожидание unchoke.
// Используй для скачивания одного куска; для нескольких — Handshake+WaitUnchoke один раз, затем DownloadPieceRaw.
func DownloadPiece(c *PeerConn, index uint32, size int64, expected [20]byte) ([]byte, error) {
	c.conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.conn.SetDeadline(time.Time{})

	if err := c.SendMessage(NewMsgInterested()); err != nil {
		return nil, fmt.Errorf("send interested: %w", err)
	}

	if err := waitUnchoke(c); err != nil {
		return nil, err
	}

	return DownloadPieceRaw(c, index, size, expected)
}

// DownloadPieceRaw скачивает кусок без повторного interested/unchoke.
// Вызывай когда соединение уже в состоянии unchoked.
func DownloadPieceRaw(c *PeerConn, index uint32, size int64, expected [20]byte) ([]byte, error) {
	buf, err := downloadBlocks(c, index, size)
	if err != nil {
		return nil, err
	}
	if err := verifyPiece(buf, expected); err != nil {
		return nil, err
	}
	return buf, nil
}

// waitUnchoke читает сообщения пока не получит unchoke.
func waitUnchoke(c *PeerConn) error {
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return fmt.Errorf("waiting unchoke: %w", err)
		}
		if msg == nil { // keep-alive
			continue
		}
		switch msg.ID {
		case MsgUnchoke:
			return nil
		case MsgChoke:
			return fmt.Errorf("peer choked us")
		case MsgHave, MsgBitfield:
			// игнорируем пока ждём unchoke
		default:
			return fmt.Errorf("unexpected message %d while waiting unchoke", msg.ID)
		}
	}
}

// downloadBlocks запрашивает все блоки куска и собирает их в буфер.
func downloadBlocks(c *PeerConn, index uint32, size int64) ([]byte, error) {
	buf := make([]byte, size)
	received := int64(0)

	for received < size {
		begin := received
		length := int64(blockSize)
		if begin+length > size {
			length = size - begin
		}

		req := NewMsgRequest(index, uint32(begin), uint32(length))
		if err := c.SendMessage(req); err != nil {
			return nil, fmt.Errorf("send request (begin=%d): %w", begin, err)
		}

		block, err := readBlock(c, index)
		if err != nil {
			return nil, err
		}

		if int64(block.Begin)+int64(len(block.Data)) > size {
			return nil, fmt.Errorf("block out of bounds: begin=%d len=%d piece size=%d",
				block.Begin, len(block.Data), size)
		}

		copy(buf[block.Begin:], block.Data)
		received += int64(len(block.Data))
	}

	return buf, nil
}

// readBlock читает сообщения пока не получит MsgPiece для нужного куска.
func readBlock(c *PeerConn, index uint32) (*PieceBlock, error) {
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read block: %w", err)
		}
		if msg == nil { // keep-alive
			continue
		}
		switch msg.ID {
		case MsgPiece:
			block, err := ParsePiece(msg)
			if err != nil {
				return nil, err
			}
			if block.Index != index {
				return nil, fmt.Errorf("got piece %d, want %d", block.Index, index)
			}
			return block, nil
		case MsgChoke:
			return nil, fmt.Errorf("peer choked mid-download")
		case MsgHave:
			// игнорируем
		}
	}
}

func verifyPiece(data []byte, expected [20]byte) error {
	got := sha1.Sum(data)
	if got != expected {
		return fmt.Errorf("SHA1 mismatch: got %x, want %x", got, expected)
	}
	return nil
}
