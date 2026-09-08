//go:build linux

package tray

import (
	"bytes"
	"image/color"
	"image/png"

	"jeemi/internal/platform/icons"
)

func prepareTrayIcon(icon []byte) ([]byte, error) {
	// StatusNotifier sends raw ARGB pixels over D-Bus. Never send the 1254px
	// application source (over 6 MB per property read) to the desktop panel.
	return icons.ResizePNG(icon, 64)
}

// StatusNotifier a(iiay): signed 32-bit dimensions and straight-alpha ARGB32
// bytes in network order. Color.RGBA() returns premultiplied 16-bit channels;
// casting its low bytes corrupts translucent pixels, including most of our PNG.
type linuxPixmap struct {
	Width, Height int32
	Pixels        []byte
}

func linuxIconPixmaps(preparedPNG []byte) []linuxPixmap {
	config, err := png.DecodeConfig(bytes.NewReader(preparedPNG))
	if err != nil || config.Width != 64 || config.Height != 64 {
		return nil
	}
	icon, err := png.Decode(bytes.NewReader(preparedPNG))
	if err != nil {
		return nil
	}
	pixels := make([]byte, 64*64*4)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			pixel := color.NRGBAModel.Convert(icon.At(x, y)).(color.NRGBA)
			offset := (y*64 + x) * 4
			pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = pixel.A, pixel.R, pixel.G, pixel.B
		}
	}
	return []linuxPixmap{{Width: 64, Height: 64, Pixels: pixels}}
}
