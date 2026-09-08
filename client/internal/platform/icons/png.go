// Package icons derives desktop-sized images from Jeemi's embedded PNG source.
package icons

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// ResizePNG uses an area average in premultiplied colour space so transparent
// edges stay smooth. Only embedded, bounded, square application icons belong here.
func ResizePNG(source []byte, size int) ([]byte, error) {
	if len(source) > 8<<20 || size < 1 || size > 512 {
		return nil, fmt.Errorf("invalid application icon size")
	}
	config, err := png.DecodeConfig(bytes.NewReader(source))
	if err != nil || config.Width != config.Height || config.Width < size || config.Width > 4096 {
		return nil, fmt.Errorf("invalid application PNG dimensions")
	}
	input, err := png.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("decode application icon: %w", err)
	}
	output := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			left, right := x*config.Width/size, (x+1)*config.Width/size
			top, bottom := y*config.Height/size, (y+1)*config.Height/size
			var r, g, b, a uint64
			for sy := top; sy < bottom; sy++ {
				for sx := left; sx < right; sx++ {
					pr, pg, pb, pa := input.At(sx, sy).RGBA()
					r, g, b, a = r+uint64(pr), g+uint64(pg), b+uint64(pb), a+uint64(pa)
				}
			}
			count := uint64((right - left) * (bottom - top))
			output.SetRGBA(x, y, color.RGBA{uint8(r / count >> 8), uint8(g / count >> 8), uint8(b / count >> 8), uint8(a / count >> 8)})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, output); err != nil {
		return nil, fmt.Errorf("encode application icon: %w", err)
	}
	return encoded.Bytes(), nil
}
