//go:build linux

package coreauth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxMemoryServiceSamplesOnlyCallersMihomoChild(t *testing.T) {
	if os.Getenv("JEEMI_MEMORY_TEST_CHILD") == "1" {
		fmt.Fprintln(os.Stdout, "ready")
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	path := filepath.Join(t.TempDir(), "mihomo")
	target, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatal(copyErr, closeErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, "-test.run=^TestLinuxMemoryServiceSamplesOnlyCallersMihomoChild$")
	command.Env = append(os.Environ(), "JEEMI_MEMORY_TEST_CHILD=1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		input.Close()
		if err := command.Wait(); err != nil {
			t.Error(err)
		}
	}()
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatal("child not ready", err)
	}
	pid := command.Process.Pid
	memory := serviceMemory(os.Getpid(), uint32(os.Getuid()), pid, false)
	if memory.HelperBytes == nil || *memory.HelperBytes == 0 || memory.MihomoBytes == nil || *memory.MihomoBytes == 0 || memory.MihomoPID != pid {
		t.Fatalf("live child memory unavailable: %+v", memory)
	}
	if serviceMemory(os.Getpid()+1, uint32(os.Getuid()), pid, false).MihomoBytes != nil {
		t.Fatal("foreign parent accepted")
	}
	if serviceMemory(os.Getpid(), uint32(os.Getuid()+1), pid, false).MihomoBytes != nil {
		t.Fatal("foreign account accepted")
	}
}

func TestMemoryPIDMustBeAuthenticatedClientsDirectChild(t *testing.T) {
	stat := func(pid, parent int) string {
		return fmt.Sprintf("%d (mihomo (name)) S %d %s", pid, parent, strings.Repeat("0 ", 20))
	}
	if !directMemoryChild(stat(11, 10), 11, 10) {
		t.Fatal("direct child rejected")
	}
	for _, input := range []string{stat(11, 20), stat(12, 10), "11 (mihomo) S 10", "bad", stat(11, 0)} {
		if directMemoryChild(input, 11, 10) {
			t.Fatal("foreign or malformed process accepted")
		}
	}
	if directMemoryChild(stat(10, 10), 10, 10) {
		t.Fatal("client counted as core")
	}
}

func TestMemoryUsesKernelUIDsForNondumpableCapabilityCore(t *testing.T) {
	if !memoryProcessUID("Name:\tmihomo\nUid:\t1000\t1000\t1000\t1000\n", 1000) {
		t.Fatal("ordinary capability-bearing core rejected")
	}
	for _, status := range []string{"", "Uid: 0 0 0 0", "Uid: 1000 0 0 0", "Uid: 1000 1000", "Uid: a b c d"} {
		if memoryProcessUID(status, 1000) {
			t.Fatal("foreign UID accepted")
		}
	}
}

func TestMemoryRequestCannotCarryPathsCommandsOrOtherOperations(t *testing.T) {
	for _, input := range []string{
		`{"protocol":2,"operation":"memory","mihomoPID":42,"webViewMemory":true}`,
		`{"protocol":2,"operation":"memory"}`,
	} {
		if _, err := decodeServiceRequest([]byte(input)); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []string{
		`{"protocol":2,"operation":"memory","mihomoPID":-1}`,
		`{"protocol":2,"operation":"memory","mihomoPID":2147483648}`,
		`{"protocol":2,"operation":"memory","target":{}}`,
		`{"protocol":2,"operation":"memory","pid":1}`,
		`{"protocol":2,"operation":"status","webViewMemory":true}`,
	} {
		if _, err := decodeServiceRequest([]byte(input)); err == nil {
			t.Fatal("unsafe request accepted")
		}
	}
}
