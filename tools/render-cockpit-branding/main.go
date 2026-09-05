// Render Cockpit icon derivatives from the canonical Soda symbol. Run from the repository root.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func main() {
	out := flag.String("out", "assets/branding/cockpit", "output directory")
	flag.Parse()
	if err := render(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func render(out string) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	var frames [][]byte
	for _, size := range []int{16, 32, 48, 180} {
		background := "none"
		name := fmt.Sprintf("favicon-%d.png", size)
		if size == 180 {
			background = "#06245B"
			name = "apple-touch-icon.png"
		}
		cmd := exec.Command("rsvg-convert", "--format=png", "--width="+strconv.Itoa(size),
			"--height="+strconv.Itoa(size), "--keep-aspect-ratio", "--background-color="+background,
			"assets/branding/source/soda-symbol.svg")
		cmd.Stderr = os.Stderr
		data, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("render %s (requires librsvg): %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
			return err
		}
		if size != 180 {
			frames = append(frames, data)
		}
	}
	return os.WriteFile(filepath.Join(out, "favicon.ico"), icon(frames), 0644)
}

// ICO permits PNG-encoded frames. Write 16, 32, and 48px entries without resampling.
func icon(frames [][]byte) []byte {
	var out bytes.Buffer
	out.Write([]byte{0, 0, 1, 0, 3, 0})
	offset := uint32(6 + 3*16)
	for i, frame := range frames {
		entry := make([]byte, 16)
		entry[0], entry[1] = byte((i+1)*16), byte((i+1)*16)
		binary.LittleEndian.PutUint16(entry[4:], 1)
		binary.LittleEndian.PutUint16(entry[6:], 32)
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(frame)))
		binary.LittleEndian.PutUint32(entry[12:], offset)
		out.Write(entry)
		offset += uint32(len(frame))
	}
	for _, frame := range frames {
		out.Write(frame)
	}
	return out.Bytes()
}
