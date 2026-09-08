package configtransfer

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/localscript"
)

func TestScriptPackageRoundTripAndStrictFormat(t *testing.T) {
	p := FromScript(localscript.Script{Summary: localscript.Summary{Name: "脚本", Description: "完整导出"}, Contents: "// 注释\nfunction main(config) { return config; }\n"})
	data, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data, ScriptKind)
	if err != nil || !reflect.DeepEqual(p, decoded) {
		t.Fatal("package changed", err)
	}
	cases := map[string][]byte{
		"version":    bytes.Replace(data, []byte(`"version": 1`), []byte(`"version": 99`), 1),
		"duplicate":  bytes.Replace(data, []byte(`"version": 1`), []byte(`"version": 1, "Version": 1`), 1),
		"unknown":    bytes.Replace(data, []byte(`"version": 1`), []byte(`"version": 1, "path": "../outside"`), 1),
		"trailing":   append(append([]byte{}, data...), []byte(`{}`)...),
		"both kinds": bytes.Replace(data, []byte(`"version": 1`), []byte(`"version": 1, "config": {}`), 1),
		"too large":  bytes.Repeat([]byte(" "), MaxBytes+1),
		"too deep":   []byte(strings.Repeat("[", 50) + "0" + strings.Repeat("]", 50)),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(input, ScriptKind); err == nil {
				t.Fatal("invalid package accepted")
			}
		})
	}
	if _, err := Decode(data, ConfigKind); err == nil {
		t.Fatal("script accepted as config")
	}
}

func TestPackageFilesAreBoundedAndRequireExtension(t *testing.T) {
	dir := t.TempDir()
	data, err := Encode(FromScript(localscript.Script{Contents: "function main(c) {return c;}"}))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "backup.json")
	if err := WriteFile(path, data); err != nil {
		t.Fatal(err)
	}
	read, err := ReadFile(path)
	if err != nil || !bytes.Equal(data, read) {
		t.Fatal("file roundtrip", err)
	}
	for _, name := range []string{"backup.yaml", "backup", "backup.json.js", "backup.jmcfg"} {
		if err := WriteFile(filepath.Join(dir, name), data); err == nil {
			t.Fatal("accepted export extension", name)
		}
		if _, err := ReadFile(filepath.Join(dir, name)); err == nil {
			t.Fatal("accepted import extension", name)
		}
	}
	if err := WriteFile(path, bytes.Repeat([]byte("x"), MaxBytes+1)); err == nil {
		t.Fatal("oversized export accepted")
	}
	read, _ = os.ReadFile(path)
	if !bytes.Equal(read, data) {
		t.Fatal("rejected export overwrote previous file")
	}
	if err := WriteFile(path, []byte("replacement")); err != nil {
		t.Fatal("replace export", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatal("temporary export leaked")
	}
	if got := SuggestedFilename(`../CON:<test>?`); filepath.Base(got) != got || !strings.HasSuffix(got, Extension) || strings.ContainsAny(got, `<>:"/\|?*`) {
		t.Fatal("unsafe suggested filename", got)
	}
}
