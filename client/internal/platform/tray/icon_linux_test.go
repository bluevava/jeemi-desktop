//go:build linux

package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestLinuxIconUsesStraightAlphaARGB(t *testing.T) {
	input := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	input.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	input.SetNRGBA(1, 0, color.NRGBA{R: 230, G: 175, B: 91, A: 253})
	input.SetNRGBA(2, 0, color.NRGBA{R: 80, G: 140, B: 200, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, input); err != nil {
		t.Fatal(err)
	}
	pixmaps := linuxIconPixmaps(encoded.Bytes())
	if len(pixmaps) != 1 || pixmaps[0].Width != 64 || pixmaps[0].Height != 64 || len(pixmaps[0].Pixels) != 64*64*4 {
		t.Fatal("invalid StatusNotifier pixmap dimensions")
	}
	want := []byte{128, 200, 100, 50, 253, 230, 175, 91, 255, 80, 140, 200, 0, 0, 0, 0}
	if !bytes.Equal(pixmaps[0].Pixels[:16], want) {
		t.Fatalf("ARGB bytes = %v, want %v", pixmaps[0].Pixels[:16], want)
	}
	if linuxIconPixmaps([]byte("not PNG")) != nil {
		t.Fatal("invalid PNG accepted")
	}
	encoded.Reset()
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 1254, 1254))); err != nil {
		t.Fatal(err)
	}
	if linuxIconPixmaps(encoded.Bytes()) != nil {
		t.Fatal("original oversized PNG accepted for D-Bus transport")
	}
}
