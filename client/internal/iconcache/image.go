package iconcache

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"regexp"
	"strings"

	_ "golang.org/x/image/webp"
)

var svgURLReference = regexp.MustCompile(`(?i)url\s*\(\s*["']?\s*([^\s"')]+)`)

func safeSVGReferences(value string) bool {
	if strings.Contains(strings.ToLower(value), "@import") {
		return false
	}
	for _, match := range svgURLReference.FindAllStringSubmatch(value, -1) {
		if !strings.HasPrefix(match[1], "#") {
			return false
		}
	}
	return true
}

func imageType(contents []byte) (string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(contents))
	if err == nil {
		if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || int64(config.Width)*int64(config.Height) > 4<<20 {
			return "", ErrUnavailable
		}
		if _, _, err := image.Decode(bytes.NewReader(contents)); err != nil {
			return "", ErrUnavailable
		}
		if format == "jpeg" {
			return "image/jpeg", nil
		}
		return "image/" + format, nil
	}
	if len(contents) >= 6 && bytes.Equal(contents[:4], []byte{0, 0, 1, 0}) {
		count := int(binary.LittleEndian.Uint16(contents[4:6]))
		if count < 1 || count > 64 || len(contents) < 6+16*count {
			return "", ErrUnavailable
		}
		for index := 0; index < count; index++ {
			entry := contents[6+16*index : 6+16*(index+1)]
			size := uint64(binary.LittleEndian.Uint32(entry[8:12]))
			offset := uint64(binary.LittleEndian.Uint32(entry[12:16]))
			if size == 0 || offset < uint64(6+16*count) || offset+size > uint64(len(contents)) {
				return "", ErrUnavailable
			}
		}
		return "image/x-icon", nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(contents))
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", ErrUnavailable
		}
		switch value := token.(type) {
		case xml.Directive:
			return "", ErrUnavailable
		case xml.ProcInst:
			if value.Target != "xml" {
				return "", ErrUnavailable
			}
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || value.Name.Local != "svg" {
					return "", ErrUnavailable
				}
			}
			depth++
			if depth > 64 || strings.EqualFold(value.Name.Local, "script") || strings.EqualFold(value.Name.Local, "foreignObject") {
				return "", ErrUnavailable
			}
			for _, attr := range value.Attr {
				if !safeSVGReferences(attr.Value) {
					return "", ErrUnavailable
				}
				if strings.HasPrefix(strings.ToLower(attr.Name.Local), "on") || (strings.EqualFold(attr.Name.Local, "href") && !strings.HasPrefix(strings.TrimSpace(attr.Value), "#")) {
					return "", ErrUnavailable
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if !safeSVGReferences(string(value)) {
				return "", ErrUnavailable
			}
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return "", ErrUnavailable
			}
		}
	}
	if roots != 1 || depth != 0 {
		return "", ErrUnavailable
	}
	return "image/svg+xml", nil
}
