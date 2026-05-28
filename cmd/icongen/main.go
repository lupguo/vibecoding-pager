package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	size := 1024
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Squircle mask (superellipse, n=4 for macOS-like rounded rect)
			nx := float64(x-size/2) / float64(size/2-40) // 40px margin for shadow space
			ny := float64(y-size/2) / float64(size/2-40)
			d := math.Pow(math.Abs(nx), 4.5) + math.Pow(math.Abs(ny), 4.5)
			if d > 1.0 {
				img.Set(x, y, color.Transparent)
				continue
			}

			// Gradient: #007aff (0,122,255) -> #5856d6 (88,86,214) at 135 degrees
			t := (float64(x)/float64(size) + float64(y)/float64(size)) / 2.0
			r := uint8(float64(0x00) + t*float64(0x58-0x00))
			g := uint8(float64(0x7a) + t*float64(0x56-0x7a))
			b := uint8(float64(0xff) + t*float64(0xd6-0xff))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// Draw a simple white "P" in the center (blocky style for icon clarity)
	drawP(img, size)

	os.MkdirAll("assets", 0o755)
	os.MkdirAll("build", 0o755)

	// Save 1024px master
	save(img, "assets/icon.png")
	save(img, "build/appicon.png")

	// Generate tray icons (simple resize for now)
	tray2x := resize(img, 44)
	save(tray2x, "assets/tray-icon@2x.png")

	tray1x := resize(img, 22)
	save(tray1x, "assets/tray-icon.png")
}

func drawP(img *image.RGBA, size int) {
	// Draw a bold "P" letter centered
	// Simple pixel approach: P = vertical bar + top half right bump
	cx, cy := size/2, size/2
	white := color.RGBA{255, 255, 255, 255}

	// Letter dimensions (relative to 1024)
	barW := size / 10       // vertical bar width
	barH := size * 45 / 100 // vertical bar height
	bumpW := size / 5       // bump width
	bumpH := size / 5       // bump height
	bumpR := bumpH / 2      // bump corner radius

	// Vertical bar of P
	for y := cy - barH/2; y < cy+barH/2; y++ {
		for x := cx - barW/2 - bumpW/4; x < cx-barW/2-bumpW/4+barW; x++ {
			img.Set(x, y, white)
		}
	}

	// Top bump of P (rounded rectangle to the right)
	bumpLeft := cx - barW/2 - bumpW/4
	bumpTop := cy - barH/2
	bumpRight := bumpLeft + barW + bumpW
	bumpBottom := bumpTop + bumpH*2

	for y := bumpTop; y < bumpBottom; y++ {
		for x := bumpLeft; x < bumpRight; x++ {
			// Only draw the outline (thick border)
			isTop := y < bumpTop+barW
			isBottom := y > bumpBottom-barW
			isRight := x > bumpRight-barW
			isInner := x > bumpLeft+barW && y > bumpTop+barW && y < bumpBottom-barW && x < bumpRight-barW

			if !isInner || isTop || isBottom || isRight {
				// Round the right corners
				if x > bumpRight-bumpR && y < bumpTop+bumpR {
					dx := float64(x - (bumpRight - bumpR))
					dy := float64(y - (bumpTop + bumpR))
					if math.Sqrt(dx*dx+dy*dy) > float64(bumpR) {
						continue
					}
				}
				if x > bumpRight-bumpR && y > bumpBottom-bumpR {
					dx := float64(x - (bumpRight - bumpR))
					dy := float64(y - (bumpBottom - bumpR))
					if math.Sqrt(dx*dx+dy*dy) > float64(bumpR) {
						continue
					}
				}
				img.Set(x, y, white)
			}
		}
	}
}

func resize(src *image.RGBA, newSize int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, newSize, newSize))
	srcSize := src.Bounds().Dx()
	scale := float64(srcSize) / float64(newSize)

	for y := 0; y < newSize; y++ {
		for x := 0; x < newSize; x++ {
			srcX := int(float64(x) * scale)
			srcY := int(float64(y) * scale)
			if srcX >= srcSize {
				srcX = srcSize - 1
			}
			if srcY >= srcSize {
				srcY = srcSize - 1
			}
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func save(img *image.RGBA, path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, img)
}
