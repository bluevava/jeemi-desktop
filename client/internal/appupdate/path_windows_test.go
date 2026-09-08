package appupdate

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// GitHub's Windows runner exposes TEMP under a short profile name (RUNNER~1).
// Use a temporary directory's actual 8.3 alias without changing system settings.
func aliasedTempDir(t *testing.T) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "Runner Profile With Long Name")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	n, err := windows.GetShortPathName(name, &buffer[0], uint32(len(buffer)))
	if err != nil || n == 0 || n >= uint32(len(buffer)) {
		t.Fatalf("resolve temporary short path: length=%d, error=%v", n, err)
	}
	alias := windows.UTF16ToString(buffer[:n])
	if alias == canonicalTestPath(t, directory) {
		t.Skip("temporary filesystem does not provide 8.3 directory aliases")
	}
	return alias
}
