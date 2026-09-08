package files

import "testing"

func TestOpenDirectoryRejectsRelativePathWithoutLaunchingFileManager(t *testing.T) {
	if err := OpenDirectory("core/mihomo"); err == nil {
		t.Fatal("OpenDirectory() accepted a relative path")
	}
}
