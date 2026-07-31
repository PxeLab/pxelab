//go:build ignore

// PxeLab 应用图标生成器：绘制 Variant B「Stream」标志
// （5 个像素块拼成尖括号 + 光标块，64 设计网格），
// 输出多尺寸 app.ico（16/32/48/256 PNG entries）、icon.rc，
// 以及 icon-preview.png（256px，供人工核对）。
//
// 用法：cd cmd/pxelab && go run gen_icon.go && windres -O coff -o icon.syso icon.rc
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

var (
	bg = color.NRGBA{0x3B, 0x82, 0xF6, 0xFF} // ocean #3B82F6
	wt = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
)

// Variant B 几何（64×64 设计网格）
var blocks = [][2]int{{8, 8}, {16, 16}, {24, 24}, {16, 32}, {8, 40}} // 尖括号像素块，9×9

const (
	blockSize = 9
	cursorX   = 38
	cursorY   = 42
	cursorW   = 18
	cursorH   = 10
	cornerR   = 14 // 圆角半径，≈22%（64 网格）
)

// drawMark 在 s×s 画布上绘制：ocean 圆角方底 + 白色 mark。
// 像素风标志，硬边绘制，无需抗锯齿。
func drawMark(s int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	scale := func(v int) int { return (v*s + 32) / 64 } // 64 网格 → 画布，四舍五入

	r := scale(cornerR)
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
			img.SetNRGBA(x, y, bg)
		}
	}

	rect := func(gx, gy, gw, gh int) {
		x0, y0 := scale(gx), scale(gy)
		x1, y1 := scale(gx+gw), scale(gy+gh)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if x >= 0 && x < s && y >= 0 && y < s {
					img.SetNRGBA(x, y, wt)
				}
			}
		}
	}
	for _, b := range blocks {
		rect(b[0], b[1], blockSize, blockSize)
	}
	rect(cursorX, cursorY, cursorW, cursorH)
	return img
}

func pngBytes(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func main() {
	sizes := []int{16, 32, 48, 256}
	images := make([][]byte, len(sizes))
	for i, s := range sizes {
		images[i] = pngBytes(drawMark(s))
	}

	// 组装 .ico：header + N 个 entry + 各 PNG 数据
	ico := &bytes.Buffer{}
	binary.Write(ico, binary.LittleEndian, uint16(0))          // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))          // type = icon
	binary.Write(ico, binary.LittleEndian, uint16(len(sizes))) // count
	offset := uint32(6 + 16*len(sizes))
	for i, s := range sizes {
		w := uint8(s)
		if s >= 256 {
			w = 0 // 0 表示 256
		}
		binary.Write(ico, binary.LittleEndian, w)                      // width
		binary.Write(ico, binary.LittleEndian, w)                      // height
		binary.Write(ico, binary.LittleEndian, uint8(0))               // colors
		binary.Write(ico, binary.LittleEndian, uint8(0))               // reserved
		binary.Write(ico, binary.LittleEndian, uint16(1))              // planes
		binary.Write(ico, binary.LittleEndian, uint16(32))             // bpp
		binary.Write(ico, binary.LittleEndian, uint32(len(images[i]))) // size
		binary.Write(ico, binary.LittleEndian, offset)                 // offset
		offset += uint32(len(images[i]))
	}
	for _, data := range images {
		ico.Write(data)
	}

	if err := os.WriteFile("app.ico", ico.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated app.ico:", len(ico.Bytes()), "bytes,", len(sizes), "sizes")

	if err := os.WriteFile("icon.rc", []byte("100 ICON \"app.ico\"\n"), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated icon.rc")

	if err := os.WriteFile("icon-preview.png", images[len(images)-1], 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated icon-preview.png (256px, for review)")
}
