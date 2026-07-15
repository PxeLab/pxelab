//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

type nrgb struct{ r, g, b, a uint8 }

func main() {
	const s = 32
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	bg := nrgb{52, 119, 235, 255}
	wt := nrgb{255, 255, 255, 255}

	// Rounded-rectangle background
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			dx, dy := 0, 0
			if x < 6 {
				dx = 6 - x
			} else if x >= s-6 {
				dx = x - (s - 6 - 1)
			}
			if y < 6 {
				dy = 6 - y
			} else if y >= s-6 {
				dy = y - (s - 6 - 1)
			}
			if dx*dx+dy*dy > 25 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{bg.r, bg.g, bg.b, bg.a})
		}
	}
	for y := 6; y < s-6; y++ {
		for x := 6; x < s-6; x++ {
			img.SetNRGBA(x, y, color.NRGBA{bg.r, bg.g, bg.b, bg.a})
		}
	}
	play := func(x, y int) {
		if x >= 0 && x < s && y >= 0 && y < s {
			img.SetNRGBA(x, y, color.NRGBA{wt.r, wt.g, wt.b, wt.a})
		}
	}
	for row := 0; row < 11; row++ {
		l, r := 10+row/2, 22
		if row >= 5 {
			r = 22 - (row-4)/2
		}
		for col := l; col <= r; col++ {
			play(col, 10+row)
		}
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		panic(err)
	}
	pngData := pngBuf.Bytes()

	// Build .ico file
	ico := &bytes.Buffer{}
	binary.Write(ico, binary.LittleEndian, uint16(0))    // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))    // type = icon
	binary.Write(ico, binary.LittleEndian, uint16(1))    // count = 1
	entryOff := uint32(6 + 16)
	binary.Write(ico, binary.LittleEndian, uint8(s))     // width
	binary.Write(ico, binary.LittleEndian, uint8(s))     // height
	binary.Write(ico, binary.LittleEndian, uint8(0))     // colors
	binary.Write(ico, binary.LittleEndian, uint8(0))     // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))    // planes
	binary.Write(ico, binary.LittleEndian, uint16(32))   // bpp
	binary.Write(ico, binary.LittleEndian, uint32(len(pngData))) // size
	binary.Write(ico, binary.LittleEndian, entryOff)     // offset
	ico.Write(pngData)

	if err := os.WriteFile("app.ico", ico.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated app.ico:", len(ico.Bytes()), "bytes")

	// Build .rc file
	rc := "100 ICON \"app.ico\"\n"
	if err := os.WriteFile("icon.rc", []byte(rc), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated icon.rc")
}
