package subscription

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/makiuchi-d/gozxing"
	multiqrcode "github.com/makiuchi-d/gozxing/multi/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode"
)

const (
	maxQRCodeImageBytes  = 20 << 20
	maxQRCodeImagePixels = 50_000_000
)

var ErrNoQRCode = errors.New("screen_qr_not_found")

type QRDecoder interface {
	Decode(image.Image) ([]string, error)
}

type ZXingQRDecoder struct{}

func (ZXingQRDecoder) Decode(source image.Image) ([]string, error) {
	bitmap, err := gozxing.NewBinaryBitmapFromImage(source)
	if err != nil {
		return nil, fmt.Errorf("prepare QR code image")
	}
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	results, multiErr := multiqrcode.NewQRCodeMultiReader().DecodeMultiple(bitmap, hints)
	texts := make([]string, 0, len(results))
	for _, result := range results {
		if text := strings.TrimSpace(result.GetText()); text != "" {
			texts = append(texts, text)
		}
	}
	if len(texts) > 0 {
		return texts, nil
	}
	reader := qrcode.NewQRCodeReader()
	result, singleErr := reader.Decode(bitmap, hints)
	if singleErr == nil {
		if text := strings.TrimSpace(result.GetText()); text != "" {
			return []string{text}, nil
		}
	}
	if multiErr != nil || singleErr != nil {
		return nil, fmt.Errorf("no readable QR code was found")
	}
	return nil, fmt.Errorf("QR code content is empty")
}

func loadQRCodeImage(path string) (image.Image, error) {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".gif" {
		return nil, fmt.Errorf("QR code image format is unsupported")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxQRCodeImageBytes {
		return nil, fmt.Errorf("QR code image is unavailable or too large")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read QR code image")
	}
	configuration, _, err := image.DecodeConfig(bytes.NewReader(contents))
	if err != nil || configuration.Width < 1 || configuration.Height < 1 || uint64(configuration.Width)*uint64(configuration.Height) > maxQRCodeImagePixels {
		return nil, fmt.Errorf("QR code image dimensions are invalid or too large")
	}
	decoded, _, err := image.Decode(bytes.NewReader(contents))
	if err != nil {
		return nil, fmt.Errorf("decode QR code image")
	}
	return decoded, nil
}
