package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestApplicationIconsHaveBoundedPixels(t *testing.T) {
	source, err := os.ReadFile("../../../build/appicon.png")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{64, 256} {
		result, err := ResizePNG(source, size)
		if err != nil {
			t.Fatal(err)
		}
		icon, err := png.Decode(bytes.NewReader(result))
		if err != nil {
			t.Fatal(err)
		}
		if icon.Bounds().Dx() != size || icon.Bounds().Dy() != size {
			t.Fatal("incorrect icon size")
		}
		_, _, _, alpha := icon.At(size/2, size/2).RGBA()
		if alpha == 0 {
			t.Fatal("application icon became invisible")
		}
	}
}

func TestResizePreservesTransparentEdges(t *testing.T) {
	input := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	input.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, input); err != nil {
		t.Fatal(err)
	}
	result, err := ResizePNG(encoded.Bytes(), 1)
	if err != nil {
		t.Fatal(err)
	}
	icon, err := png.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	pixel := color.NRGBAModel.Convert(icon.At(0, 0)).(color.NRGBA)
	if pixel.R != 255 || pixel.G != 0 || pixel.B != 0 || pixel.A != 63 {
		t.Fatalf("bad edge colour: %+v", pixel)
	}
	if _, err := ResizePNG([]byte("not a PNG"), 64); err == nil {
		t.Fatal("invalid PNG accepted")
	}
	if _, err := ResizePNG(encoded.Bytes(), 4096); err == nil {
		t.Fatal("unbounded size accepted")
	}
}
