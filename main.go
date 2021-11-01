package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"unicode/utf8"
)

type Header struct {
	Magic          uint32
	Version        uint32
	HeaderSize     uint32
	Flags          uint32
	NumberOfGlyphs uint32
	BytesPerGlyph  uint32
	Height         uint32
	Width          uint32
}

func (h Header) String() string {
	return fmt.Sprintf("<Magic: 0x%08x, Version: %d, Header: %d, Flags: 0x%08x, Glyphs: %d, Bytes per glyph: %d, Size: %dx%d >",
		h.Magic, h.Version, h.HeaderSize, h.Flags, h.NumberOfGlyphs, h.BytesPerGlyph, h.Width, h.Height)
}

func main() {

	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s [path to PSF file] [path for PNG file]", os.Args[0])
		return
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("Failed to open %s: %v\n", os.Args[1], err)
		return
	}
	defer f.Close()

	header := Header{}
	err = binary.Read(f, binary.LittleEndian, &header)
	if err != nil {
		fmt.Printf("Failed to read header: %v\n", err)
		return
	}

	fmt.Printf("Header: %v", header)

	if header.BytesPerGlyph < ((header.Width * header.Height) / 8) {
		fmt.Printf("Glyph data seems to be too small\n")
		return
	}

	glyphmap := make([][]rune, header.NumberOfGlyphs)

	if header.Flags == 0x1 {
		f.Seek(int64(header.HeaderSize)+(int64(header.BytesPerGlyph)*int64(header.NumberOfGlyphs)), 0)
		for i := uint32(0); i < header.NumberOfGlyphs; i++ {
			runelist := glyphmap[i]

			bytes := make([]byte, 0)
			b := make([]byte, 1)
			for true {
				count, err := f.Read(b)
				if b[0] == 0xFF {
					break
				}
				if err != nil {
					fmt.Printf("Failed to read glyth table: %v", err)
					return
				}
				if count == 0 {
					fmt.Printf("Data undeflow reading glyph table")
					return
				}
				bytes = append(bytes, b[0])
			}

			for len(bytes) > 0 {
				rune, size := utf8.DecodeRune(bytes)
				if rune == utf8.RuneError {
					fmt.Printf("Failed to decode UTF8 rune in table (%d): %v", size, bytes)
					return
				}
				runelist = append(runelist, rune)
				bytes = bytes[size:]
			}

			glyphmap[i] = runelist
		}
	} else {
		for i := uint32(0); i < header.NumberOfGlyphs; i++ {
			runelist := make([]rune, 1)
			runelist[0] = rune(i)
			glyphmap[i] = runelist
		}
	}

	for i := uint32(0); i < header.NumberOfGlyphs; i++ {
		runelist := glyphmap[i]
		if len(runelist) == 0 {
			continue
		}
		fmt.Printf("0x%03x: ", i)
		for j := 0; j < len(runelist); j++ {
			fmt.Printf("%c ", runelist[j])
		}
		fmt.Printf("\n")

	}

	// go to start of glyph data in case we read the mapping table
	f.Seek(int64(header.HeaderSize), 0)

	image_width := int(1 + ((header.Width + 1) * 16))
	image_height := int(1 + ((header.Height + 1) * uint32(math.Ceil(float64(header.NumberOfGlyphs)/16.0))))

	img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{image_width, image_height}})

	background := color.RGBA{0xF0, 0xC0, 0xC0, 0xFF}
	for x := 0; x < image_width; x++ {
		for y := 0; y < image_height; y++ {
			img.Set(x, y, background)
		}
	}

	foreground := color.RGBA{0x10, 0x10, 0x10, 0xFF}
	for i := uint32(0); i < header.NumberOfGlyphs; i++ {

		glyphs := glyphmap[i]
		if len(glyphs) == 0 {
			background = color.RGBA{0xF0, 0xC0, 0xC0, 0xFF}
		} else {
			background = color.RGBA{0xF0, 0xF0, 0xF0, 0xFF}
		}

		bits := make([]byte, header.BytesPerGlyph)
		count, err := f.Read(bits)
		if err != nil {
			fmt.Printf("Failed to read glyph %d: %v\n", i, err)
			return
		}
		if count != int(header.BytesPerGlyph) {
			fmt.Printf("Author needs to be less lazy\n")
			return
		}

		x_offset := ((i % 16) * (header.Width + 1)) + 1
		y_offset := ((i / 16) * (1 + header.Height)) + 1

		index := 0
		data := byte(0)
		for y := uint32(0); y < header.Height; y++ {
			for x := uint32(0); x < header.Width; x++ {
				if x%8 == 0 {
					data = bits[index]
					index += 1
				}
				if (data & 0x80) == 0x80 {
					img.Set(int(x_offset+(x)), int(y_offset+y), foreground)
				} else {
					img.Set(int(x_offset+(x)), int(y_offset+y), background)
				}
				data <<= 1
			}
		}

	}

	image_file, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Printf("Failed to generate %s: %v\n", os.Args[2], err)
		return
	}
	defer image_file.Close()
	err = png.Encode(image_file, img)
	if err != nil {
		fmt.Printf("Failed to write png: %v\n", err)
		return
	}

}
