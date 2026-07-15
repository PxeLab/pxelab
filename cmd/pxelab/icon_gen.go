package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

type nrgba struct {
	r, g, b, a uint8
}

func generateTrayIcon() ([]byte, error) {
	const s = 32
	img := image.NewNRGBA(image.Rect(0, 0, s, s))

	bg := nrgba{52, 119, 235, 255}
	hl := nrgba{72, 149, 245, 255}
	wt := nrgba{255, 255, 255, 255}

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
			r2 := dx*dx + dy*dy
			if r2 > 25 {
				continue
			}
			c := bg
			if r2 > 16 {
				c = hl
			}
			img.SetNRGBA(x, y, color.NRGBA{c.r, c.g, c.b, c.a})
		}
	}
	// Fill center area
	for y := 6; y < s-6; y++ {
		for x := 6; x < s-6; x++ {
			img.SetNRGBA(x, y, color.NRGBA{bg.r, bg.g, bg.b, bg.a})
		}
	}

	// Draw a white "play" triangle pointing right
	for row := 0; row < 11; row++ {
		l, r := 10+row/2, 22
		if row >= 5 {
			r = 22 - (row-4)/2
		}
		for col := l; col <= r; col++ {
			img.SetNRGBA(col, 10+row, color.NRGBA{wt.r, wt.g, wt.b, wt.a})
		}
	}

	// Encode to PNG first
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}

	// Wrap PNG in ICO container (Windows requires .ico format for systray)
	ico := &bytes.Buffer{}
	binary.Write(ico, binary.LittleEndian, uint16(0))       // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))       // type = icon
	binary.Write(ico, binary.LittleEndian, uint16(1))       // count = 1
	binary.Write(ico, binary.LittleEndian, uint8(s))        // width
	binary.Write(ico, binary.LittleEndian, uint8(s))        // height
	binary.Write(ico, binary.LittleEndian, uint8(0))        // colors
	binary.Write(ico, binary.LittleEndian, uint8(0))        // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))       // planes
	binary.Write(ico, binary.LittleEndian, uint16(32))      // bpp
	pngData := pngBuf.Bytes()
	binary.Write(ico, binary.LittleEndian, uint32(len(pngData))) // size
	binary.Write(ico, binary.LittleEndian, uint32(6+16))         // offset (header + 1 entry)
	ico.Write(pngData)

	return ico.Bytes(), nil
}
