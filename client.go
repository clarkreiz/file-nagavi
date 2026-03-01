package main

import (
	"fmt"
	"net"
)

// Download скачивает все куски торрента от одного пира и записывает на диск.
func Download(tor *Torrent, peerAddr string, destDir string) error {
	peerID := NewPeerID()

	ip, portStr, err := net.SplitHostPort(peerAddr)
	if err != nil {
		return fmt.Errorf("parse peer addr: %w", err)
	}
	port := uint16(0)
	fmt.Sscanf(portStr, "%d", &port)
	peer := Peer{IP: net.ParseIP(ip), Port: port}

	fmt.Printf("Connecting to %s...\n", peerAddr)
	conn, err := Connect(peer, tor.InfoHash, peerID)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	fmt.Println("Handshake OK")

	fw, err := NewFileWriter(destDir, &tor.Info)
	if err != nil {
		return fmt.Errorf("file writer: %w", err)
	}

	if err := conn.SendMessage(NewMsgInterested()); err != nil {
		return fmt.Errorf("send interested: %w", err)
	}
	if err := waitUnchoke(conn); err != nil {
		return fmt.Errorf("wait unchoke: %w", err)
	}

	pieces := tor.Info.PieceList()
	total := len(pieces)

	for _, p := range pieces {
		fmt.Printf("\rDownloading piece %d/%d...", p.Index+1, total)

		data, err := DownloadPieceRaw(conn, uint32(p.Index), p.Size, p.Hash)
		if err != nil {
			return fmt.Errorf("piece %d: %w", p.Index, err)
		}

		if err := fw.WritePiece(uint32(p.Index), tor.Info.PieceLength, data); err != nil {
			return fmt.Errorf("write piece %d: %w", p.Index, err)
		}
	}

	fmt.Printf("\nDone. Saved to %s\n", destDir)
	return nil
}
