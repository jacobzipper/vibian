package computer

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// cropScale crops a full-screen PNG to r (layout pixels, empty = all) and
// scales it so one layout pixel becomes scale image pixels. screen is the
// layout box the PNG shows; the PNG may have more pixels than it on HiDPI.
func cropScale(data []byte, screen, r Rect, scale float64) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading the screenshot: %v", err)
	}
	sb := src.Bounds()
	if screen.Empty() {
		screen = Rect{0, 0, sb.Dx(), sb.Dy()}
	}
	if r.Empty() {
		r = screen
	}
	// image pixels per layout pixel
	kx, ky := float64(sb.Dx())/float64(screen.W), float64(sb.Dy())/float64(screen.H)
	crop := image.Rect(
		sb.Min.X+int(float64(r.X-screen.X)*kx), sb.Min.Y+int(float64(r.Y-screen.Y)*ky),
		sb.Min.X+int(float64(r.X-screen.X+r.W)*kx), sb.Min.Y+int(float64(r.Y-screen.Y+r.H)*ky),
	).Intersect(sb)
	if crop.Empty() {
		return nil, fmt.Errorf("region %d,%d %dx%d is off screen", r.X, r.Y, r.W, r.H)
	}
	w, h := max(1, int(float64(r.W)*scale+0.5)), max(1, int(float64(r.H)*scale+0.5))
	rgba := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(rgba, rgba.Bounds(), src, crop.Min, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, boxScale(rgba, w, h)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// boxScale resizes src to w×h, averaging the source pixels under each
// destination pixel (and repeating them when enlarging).
func boxScale(src *image.RGBA, w, h int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == w && sh == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		y0, y1 := y*sh/h, max((y+1)*sh/h, y*sh/h+1)
		for x := range w {
			x0, x1 := x*sw/w, max((x+1)*sw/w, x*sw/w+1)
			var sum [4]int
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					sum[0] += int(p[0])
					sum[1] += int(p[1])
					sum[2] += int(p[2])
					sum[3] += int(p[3])
				}
			}
			n := (y1 - y0) * (x1 - x0)
			d := dst.Pix[y*dst.Stride+x*4:]
			for i := range 4 {
				d[i] = uint8(sum[i] / n)
			}
		}
	}
	return dst
}
