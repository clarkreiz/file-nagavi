package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// fileRegion описывает один файл в общем пространстве торрента.
type fileRegion struct {
	path   string
	offset int64 // глобальное смещение начала файла
	length int64
}

// FileWriter записывает куски торрента на диск.
// Все файлы трактуются как один непрерывный массив байт (как в спецификации BitTorrent).
type FileWriter struct {
	regions []fileRegion
	total   int64
}

// NewFileWriter создаёт FileWriter для торрента и открывает/создаёт файлы.
func NewFileWriter(destDir string, info *Info) (*FileWriter, error) {
	var regions []fileRegion

	if info.IsMultiFile() {
		offset := int64(0)
		for _, f := range info.Files {
			p := filepath.Join(append([]string{destDir, info.Name}, f.Path...)...)
			regions = append(regions, fileRegion{path: p, offset: offset, length: f.Length})
			offset += f.Length
		}
	} else {
		p := filepath.Join(destDir, info.Name)
		regions = []fileRegion{{path: p, offset: 0, length: info.Length}}
	}

	// создаём директории и файлы нужного размера
	for _, r := range regions {
		if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(r.path), err)
		}
		f, err := os.OpenFile(r.path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", r.path, err)
		}
		if err := f.Truncate(r.length); err != nil {
			f.Close()
			return nil, fmt.Errorf("truncate %s: %w", r.path, err)
		}
		f.Close()
	}

	total := int64(0)
	if len(regions) > 0 {
		last := regions[len(regions)-1]
		total = last.offset + last.length
	}

	return &FileWriter{regions: regions, total: total}, nil
}

// WritePiece записывает кусок данных по глобальному смещению.
func (fw *FileWriter) WritePiece(pieceIndex uint32, pieceLength int64, data []byte) error {
	globalOffset := int64(pieceIndex) * pieceLength

	if globalOffset+int64(len(data)) > fw.total {
		return fmt.Errorf("piece %d exceeds total size", pieceIndex)
	}

	return fw.writeAt(globalOffset, data)
}

// writeAt записывает данные начиная с globalOffset, разбивая по файлам при необходимости.
func (fw *FileWriter) writeAt(globalOffset int64, data []byte) error {
	remaining := data

	for _, r := range fw.regions {
		if len(remaining) == 0 {
			break
		}
		// пропускаем файлы, которые заканчиваются до нашего смещения
		if r.offset+r.length <= globalOffset {
			continue
		}
		// файлы, которые начинаются после конца наших данных
		if r.offset >= globalOffset+int64(len(data)) {
			break
		}

		// смещение внутри файла
		fileOffset := globalOffset - r.offset
		if fileOffset < 0 {
			fileOffset = 0
		}

		// сколько байт пишем в этот файл
		writeLen := r.length - fileOffset
		if int64(len(remaining)) < writeLen {
			writeLen = int64(len(remaining))
		}

		if err := writeToFile(r.path, fileOffset, remaining[:writeLen]); err != nil {
			return err
		}

		remaining = remaining[writeLen:]
		globalOffset += writeLen
	}

	return nil
}

func writeToFile(path string, offset int64, data []byte) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteAt(data, offset); err != nil {
		return fmt.Errorf("write %s at %d: %w", path, offset, err)
	}
	return nil
}
