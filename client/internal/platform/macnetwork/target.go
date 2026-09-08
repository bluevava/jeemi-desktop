package macnetwork

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// Only GUI-side code reads this metadata. The root helper independently checks
// it against its own verified official download, never trusting these bytes.
func TargetForExecutable(executable string) (Target, error) {
	if !filepath.IsAbs(executable) || filepath.Base(executable) != "mihomo" {
		return Target{}, Failure("core_unverified")
	}
	file, err := os.Open(filepath.Join(filepath.Dir(executable), "metadata.json"))
	if err != nil {
		return Target{}, Failure("core_unverified")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return Target{}, Failure("core_unverified")
	}
	var metadata struct {
		Version string `json:"version"`
		SHA256  string `json:"binarySha256"`
		Size    int64  `json:"binarySize"`
	}
	if json.NewDecoder(io.LimitReader(file, 65536)).Decode(&metadata) != nil {
		return Target{}, Failure("core_unverified")
	}
	target := Target{Version: metadata.Version, SHA256: metadata.SHA256, Size: metadata.Size}
	return target, target.Validate()
}
