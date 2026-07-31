package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// 系统托盘图标：与 app.ico 同一标志（Variant B「Stream」，64 设计网格），
// 32px 单档 ICO（PNG 压缩，Windows systray 要求 .ico 容器）。
var (
	trayBg = color.NRGBA{0x3B, 0x82, 0xF6, 0xFF} // ocean #3B82F6
	trayWt = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
)

var trayBlocks = [][2]int{{8, 8}, {16, 16}, {24, 24}, {16, 32}, {8, 40}} // 尖括号像素块，9×9

func drawTrayMark(s int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	scale := func(v int) int { return (v*s + 32) / 64 }

	r := scale(14) // 圆角 ≈22%
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			dx, dy := 0, 0
			if x < r {
				dx = r - x
			} else if x >= s-r {
				dx = x - (s - r - 1)
			}
			if y < r {
				dy = r - y
			} else if y >= s-r {
				dy = y - (s - r - 1)
			}
			if dx*dx+dy*dy > r*r {
				continue
			}
			img.SetNRGBA(x, y, trayBg)
		}
	}

	rect := func(gx, gy, gw, gh int) {
		x0, y0 := scale(gx), scale(gy)
		x1, y1 := scale(gx+gw), scale(gy+gh)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if x >= 0 && x < s && y >= 0 && y < s {
					img.SetNRGBA(x, y, trayWt)
				}
			}
		}
	}
	for _, b := range trayBlocks {
		rect(b[0], b[1], 9, 9)
	}
	rect(38, 42, 18, 10) // 光标块
	return img
}

func generateTrayIcon() ([]byte, error) {
	const s = 32
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, drawTrayMark(s)); err != nil {
		return nil, err
	}
	pngData := pngBuf.Bytes()

	ico := &bytes.Buffer{}
	binary.Write(ico, binary.LittleEndian, uint16(0))            // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))            // type = icon
	binary.Write(ico, binary.LittleEndian, uint16(1))            // count = 1
	binary.Write(ico, binary.LittleEndian, uint8(s))             // width
	binary.Write(ico, binary.LittleEndian, uint8(s))             // height
	binary.Write(ico, binary.LittleEndian, uint8(0))             // colors
	binary.Write(ico, binary.LittleEndian, uint8(0))             // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))            // planes
	binary.Write(ico, binary.LittleEndian, uint16(32))           // bpp
	binary.Write(ico, binary.LittleEndian, uint32(len(pngData))) // size
	binary.Write(ico, binary.LittleEndian, uint32(6+16))         // offset (header + 1 entry)
	ico.Write(pngData)

	return ico.Bytes(), nil
}
