// Command adfextract extracts an Amiga Original File System disk image.
// It uses only the Go standard library and can be run without a go.mod:
// go run cmd/adfextract/main.go INPUT.adf OUTPUT_DIRECTORY
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const blockSize = 512

type disk struct {
	data []byte
	seen map[uint32]bool
}

func (d *disk) block(n uint32) ([]byte, error) {
	start := uint64(n) * blockSize
	if start+blockSize > uint64(len(d.data)) {
		return nil, fmt.Errorf("block %d is outside image", n)
	}
	return d.data[start : start+blockSize], nil
}

func word(b []byte, offset int) uint32 { return binary.BigEndian.Uint32(b[offset:]) }

func checkedBlock(d *disk, n uint32, expected uint32) ([]byte, error) {
	b, err := d.block(n)
	if err != nil {
		return nil, err
	}
	if word(b, 0) != expected {
		return nil, fmt.Errorf("block %d type is %d, expected %d", n, word(b, 0), expected)
	}
	var sum uint32
	for i := 0; i < blockSize; i += 4 {
		sum += word(b, i)
	}
	if sum != 0 {
		return nil, fmt.Errorf("block %d checksum failed", n)
	}
	return b, nil
}

func entryName(b []byte) (string, error) {
	n := int(b[432])
	if n < 1 || n > 30 {
		return "", fmt.Errorf("invalid filename length %d", n)
	}
	name := string(b[433 : 433+n])
	if name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("unsafe filename %q", name)
	}
	return name, nil
}

func (d *disk) file(header []byte) ([]byte, error) {
	size := int(word(header, 324))
	if size > len(d.data) {
		return nil, fmt.Errorf("file exceeds image size")
	}
	data := make([]byte, 0, size)
	seen := make(map[uint32]bool)
	n := word(header, 16)
	for n != 0 {
		if seen[n] {
			return nil, fmt.Errorf("data block cycle at %d", n)
		}
		seen[n] = true
		b, err := checkedBlock(d, n, 8)
		if err != nil {
			return nil, err
		}
		count := int(word(b, 12))
		if count > blockSize-24 || len(data)+count > size {
			return nil, fmt.Errorf("invalid data count in block %d", n)
		}
		data = append(data, b[24:24+count]...)
		n = word(b, 16)
	}
	if len(data) != size {
		return nil, fmt.Errorf("file contains %d bytes, expected %d", len(data), size)
	}
	return data, nil
}

func (d *disk) directory(n uint32, output string) error {
	b, err := checkedBlock(d, n, 2)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	for slot := 0; slot < 72; slot++ {
		child := word(b, 24+slot*4)
		for child != 0 {
			if d.seen[child] {
				return fmt.Errorf("directory cycle or duplicate entry at %d", child)
			}
			d.seen[child] = true
			entry, err := checkedBlock(d, child, 2)
			if err != nil {
				return err
			}
			name, err := entryName(entry)
			if err != nil {
				return err
			}
			path := filepath.Join(output, name)
			switch int32(word(entry, 508)) {
			case 2:
				if err := d.directory(child, path); err != nil {
					return err
				}
			case -3:
				data, err := d.file(entry)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					return err
				}
				fmt.Printf("%7d  %s\n", len(data), path)
			default:
				return fmt.Errorf("unsupported directory entry type %d", int32(word(entry, 508)))
			}
			child = word(entry, 496)
		}
	}
	return nil
}

func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: adfextract INPUT.adf OUTPUT_DIRECTORY")
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	if len(data) < 1024 || len(data)%blockSize != 0 || string(data[:4]) != "DOS\x00" {
		return fmt.Errorf("expected a DOS/0 Amiga OFS image")
	}
	d := disk{data: data, seen: make(map[uint32]bool)}
	root := word(data[:blockSize], 8)
	if root == 0 {
		root = uint32(len(data) / blockSize / 2)
	}
	return d.directory(root, os.Args[2])
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
