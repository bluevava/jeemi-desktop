package configtransfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func SuggestedFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return "Jeemi-" + name + Extension
}

func ReadFile(path string) ([]byte, error) {
	if !strings.EqualFold(filepath.Ext(path), Extension) {
		return nil, fmt.Errorf("select a .json file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open Jeemi JSON file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, fmt.Errorf("Jeemi JSON file is invalid or exceeds 16 MiB")
	}
	contents, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil || len(contents) > MaxBytes {
		return nil, fmt.Errorf("cannot read Jeemi JSON file within 16 MiB")
	}
	return contents, nil
}

// WriteFile replaces the chosen export only after the complete package is synced.
func WriteFile(path string, contents []byte) error {
	if !strings.EqualFold(filepath.Ext(path), Extension) {
		return fmt.Errorf("export filename must end in .json")
	}
	if len(contents) == 0 || len(contents) > MaxBytes {
		return fmt.Errorf("invalid Jeemi JSON export size")
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("export destination must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access export destination")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".jeemi-export-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create Jeemi JSON export")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("cannot protect Jeemi JSON export")
	}
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("cannot write Jeemi JSON export")
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("cannot sync Jeemi JSON export")
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cannot close Jeemi JSON export")
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("cannot replace Jeemi JSON export destination")
	}
	return nil
}
