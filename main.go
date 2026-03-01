package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "seed":
		cmdSeed(os.Args[2:])
	case "download":
		cmdDownload(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// seed <file> [port]
func cmdSeed(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: go-tor seed <file> [port]")
		os.Exit(1)
	}
	filePath := args[0]
	port := 6881
	if len(args) >= 2 {
		if p, err := strconv.Atoi(args[1]); err == nil {
			port = p
		}
	}

	fmt.Printf("Creating torrent for %q...\n", filePath)
	tor, err := CreateTorrent(filePath, DefaultPieceLength)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	torrentPath := filePath + ".torrent"
	if err := WriteTorrentFile(tor, torrentPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Torrent file: %s\n", torrentPath)
	fmt.Printf("InfoHash:     %x\n", tor.InfoHash)
	fmt.Printf("Pieces:       %d x %d KiB\n", len(tor.Info.Pieces), tor.Info.PieceLength/1024)
	fmt.Printf("Total size:   %d bytes\n", tor.Info.TotalLength())
	fmt.Println()
	fmt.Printf("Send %s to the other computer, then run:\n", torrentPath)
	fmt.Printf("  go-tor download %s <your-ip>:%d\n", torrentPath, port)
	fmt.Println()

	seeder, err := NewSeeder(tor, filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := seeder.Listen(port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// download <torrent> <ip:port> [dest-dir]
func cmdDownload(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go-tor download <torrent> <ip:port> [dest-dir]")
		os.Exit(1)
	}
	torrentPath := args[0]
	peerAddr := args[1]
	destDir := "."
	if len(args) >= 3 {
		destDir = args[2]
	}

	tor, err := ParseTorrent(torrentPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Name:       %s\n", tor.Info.Name)
	fmt.Printf("InfoHash:   %x\n", tor.InfoHash)
	fmt.Printf("Pieces:     %d\n", len(tor.Info.Pieces))
	fmt.Printf("Total size: %d bytes\n", tor.Info.TotalLength())
	fmt.Println()

	if err := Download(tor, peerAddr, destDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go-tor seed <file> [port]               seed a file (default port: 6881)")
	fmt.Println("  go-tor download <torrent> <ip:port> [dir]  download from peer")
}
