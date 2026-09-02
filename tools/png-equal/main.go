package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: png-equal LEFT RIGHT")
		os.Exit(2)
	}
	equal, err := equalPNG(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if !equal {
		os.Exit(1)
	}
}

func equalPNG(leftPath, rightPath string) (bool, error) {
	leftFile, err := os.Open(leftPath)
	if err != nil {
		return false, err
	}
	defer leftFile.Close()
	left, err := png.Decode(leftFile)
	if err != nil {
		return false, err
	}

	rightFile, err := os.Open(rightPath)
	if err != nil {
		return false, err
	}
	defer rightFile.Close()
	right, err := png.Decode(rightFile)
	if err != nil {
		return false, err
	}

	if left.Bounds() != right.Bounds() {
		return false, nil
	}
	return bytes.Equal(rgbaPixels(left), rgbaPixels(right)), nil
}

func rgbaPixels(source image.Image) []byte {
	decoded := image.NewRGBA(source.Bounds())
	draw.Draw(decoded, decoded.Bounds(), source, source.Bounds().Min, draw.Src)
	return decoded.Pix
}
