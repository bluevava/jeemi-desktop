//go:build linux

package coreauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"jeemi/internal/platform/requirements"

	"golang.org/x/sys/unix"
)

func fixture(t *testing.T) Target {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jeemi_data", "core", "mihomo", "v1.19.30", "linux-"+runtime.GOARCH, "mihomo")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 128)
	copy(data, "\x7fELF\x02\x01")
	machine := uint16(62)
	if runtime.GOARCH == "arm64" {
		machine = 183
	}
	binary.LittleEndian.PutUint16(data[18:20], machine)
	if err := os.WriteFile(path, data, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return Target{ExecutablePath: path, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
}

// Never call the real capability setter, even if the test suite runs as root.
type fakeGrant struct {
	flags               int64
	leaseErr, setErr    error
	broken, badReadback bool
	writes, releases    int
	value               []byte
}

func (f *fakeGrant) filesystemFlags(int) (int64, error) { return f.flags, nil }

func (f *fakeGrant) lease(int) error      { return f.leaseErr }
func (f *fakeGrant) release(int)          { f.releases++ }
func (f *fakeGrant) leaseIntact(int) bool { return !f.broken }
func (f *fakeGrant) setCapabilities(_ int, value []byte) error {
	f.writes++
	f.value = append([]byte(nil), value...)
	return f.setErr
}
func (f *fakeGrant) capabilities(int) ([]byte, error) {
	if f.badReadback {
		return nil, nil
	}
	return f.value, nil
}

func TestGrantOnlyFixedCapabilitiesOnVerifiedCore(t *testing.T) {
	target := fixture(t)
	ops := &fakeGrant{}
	if err := grant(context.Background(), target, uint32(os.Getuid()), ops); err != nil {
		t.Fatal(err)
	}
	if ops.writes != 1 || ops.releases != 1 || !requirements.HasNetworkCapabilities(ops.value) {
		t.Fatalf("bad grant: %+v", ops)
	}
	want := make([]byte, 20)
	binary.LittleEndian.PutUint32(want, 0x02000001)
	binary.LittleEndian.PutUint32(want[4:], (1<<unix.CAP_NET_ADMIN)|(1<<unix.CAP_NET_RAW)|(1<<unix.CAP_NET_BIND_SERVICE))
	if !bytes.Equal(ops.value, want) {
		t.Fatalf("unexpected capabilities: %x", ops.value)
	}
	if _, err := unix.Getxattr(target.ExecutablePath, "security.capability", nil); !errors.Is(err, unix.ENODATA) {
		t.Fatalf("test wrote real capabilities: %v", err)
	}
}

func TestGrantRejectsUnsafeOrChangedCoreBeforeWriting(t *testing.T) {
	for _, kind := range []string{"truncated", "tampered", "digest", "size", "architecture", "outside", "symlink", "parent symlink", "hardlink", "shared file", "shared directory", "owner", "lease", "lease broken", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			target := fixture(t)
			ops := &fakeGrant{}
			uid := uint32(os.Getuid())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "truncated":
				must(os.Truncate(target.ExecutablePath, 64))
			case "tampered", "architecture":
				data, err := os.ReadFile(target.ExecutablePath)
				must(err)
				index := 90
				if kind == "architecture" {
					index = 18
				}
				data[index] ^= 1
				must(os.WriteFile(target.ExecutablePath, data, 0o700))
				if kind == "architecture" {
					sum := sha256.Sum256(data)
					target.SHA256 = hex.EncodeToString(sum[:])
				}
			case "digest":
				target.SHA256 = strings.Repeat("0", 64)
			case "size":
				target.Size = maxBinarySize + 1
			case "outside":
				target.ExecutablePath = filepath.Join(t.TempDir(), "mihomo")
			case "symlink":
				other := target.ExecutablePath + ".real"
				must(os.Rename(target.ExecutablePath, other))
				must(os.Symlink(other, target.ExecutablePath))
			case "parent symlink":
				dir := filepath.Dir(target.ExecutablePath)
				must(os.Rename(dir, dir+".real"))
				must(os.Symlink(dir+".real", dir))
			case "hardlink":
				must(os.Link(target.ExecutablePath, target.ExecutablePath+".link"))
			case "shared file":
				must(os.Chmod(target.ExecutablePath, 0o777))
			case "shared directory":
				must(os.Chmod(filepath.Dir(target.ExecutablePath), 0o777))
			case "owner":
				uid++
			case "lease":
				ops.leaseErr = unix.EAGAIN
			case "lease broken":
				ops.broken = true
			case "cancelled":
				cancel()
			}
			if err := grant(ctx, target, uid, ops); err == nil || ops.writes != 0 {
				t.Fatalf("unsafe grant: err=%v writes=%d", err, ops.writes)
			}
		})
	}
}

func TestGrantReportsWriteAndReadbackFailure(t *testing.T) {
	for _, ops := range []*fakeGrant{{setErr: unix.EOPNOTSUPP}, {badReadback: true}} {
		if err := grant(context.Background(), fixture(t), uint32(os.Getuid()), ops); err == nil || ops.releases != 1 {
			t.Fatalf("grant not checked/cleaned: %+v %v", ops, err)
		}
	}
}

func TestGrantRejectsIncompatibleFilesystemBeforeWriting(t *testing.T) {
	for _, flag := range []int64{unix.ST_NOSUID, unix.ST_NOEXEC, unix.ST_RDONLY} {
		ops := &fakeGrant{flags: flag}
		err := grant(context.Background(), fixture(t), uint32(os.Getuid()), ops)
		if requirements.Code(err, "") != "linux_core_authorization_filesystem" || ops.writes != 0 {
			t.Fatalf("mount not rejected: %v %+v", err, ops)
		}
	}
}

func TestBoundedPrivateProtocol(t *testing.T) {
	target := fixture(t)
	valid, _ := json.Marshal(target)
	decoded, err := decodeTarget(bytes.NewReader(valid))
	if err != nil || decoded != target {
		t.Fatalf("decode valid: %+v %v", decoded, err)
	}
	for _, input := range []string{string(valid) + "{}", `{"command":"sh"}`, string(valid) + strings.Repeat(" ", 16<<10), "null", ""} {
		got, err := decodeTarget(strings.NewReader(input))
		// null is structurally valid JSON but must still fail before any write.
		if err == nil {
			ops := &fakeGrant{}
			err = grant(context.Background(), got, uint32(os.Getuid()), ops)
			if ops.writes != 0 {
				t.Fatal("untrusted protocol caused a write")
			}
		}
		if err == nil {
			t.Fatalf("accepted untrusted input %q", input)
		}
	}
	var output limitedOutput
	data := bytes.Repeat([]byte("x"), 9000)
	if n, _ := output.Write(data); n != len(data) || output.Len() != 4096 {
		t.Fatal("unbounded helper output")
	}
}

func TestAuthorizationResultDoesNotLeakOutputOrMisreportCancellation(t *testing.T) {
	for _, test := range []struct {
		output             string
		contextErr, runErr error
		code               string
		cancelled          bool
	}{
		{output: `{"code":"authorized"}`},
		{contextErr: context.Canceled, cancelled: true},
		{contextErr: context.DeadlineExceeded, code: "linux_core_authorization_timeout"},
		{output: "password=secret-token", code: "linux_core_authorization_failed"},
		{output: `{"code":"secret-token"}`, code: "linux_core_authorization_failed"},
		{output: `{"code":"authorized"}`, runErr: errors.New("secret-token"), code: "linux_core_authorization_failed"},
		{output: `{"code":"linux_core_integrity_failed"}`, runErr: errors.New("error"), code: "linux_core_integrity_failed"},
	} {
		err := authorizationResult(test.contextErr, test.runErr, []byte(test.output))
		if test.cancelled {
			if !errors.Is(err, ErrCancelled) {
				t.Fatal(err)
			}
			continue
		}
		if requirements.Code(err, "") != test.code || (err != nil && strings.Contains(err.Error(), "secret-token")) {
			t.Fatalf("bad result: %v", err)
		}
	}
	for _, code := range []string{"124", "126", "127"} {
		err := exec.Command("/bin/sh", "-c", "exit "+code).Run() // inert subprocess, never pkexec
		got := authorizationResult(nil, err, nil)
		if code == "124" && requirements.Code(got, "") != "linux_core_authorization_timeout" {
			t.Fatal(got)
		}
		if code == "126" && !errors.Is(got, ErrCancelled) {
			t.Fatal(got)
		}
		if code == "127" && requirements.Code(got, "") != "linux_core_authentication_failed" {
			t.Fatal(got)
		}
	}
}

func TestHelperRefusesInvocationWithoutPkexec(t *testing.T) {
	t.Setenv("PKEXEC_UID", "")
	var output bytes.Buffer
	if RunHelper(strings.NewReader("{}"), &output) == 0 || !strings.Contains(output.String(), "linux_core_authentication_failed") {
		t.Fatal(output.String())
	}
}

func TestPersistedPrivilegesSkipHelperAndPasswordPrompt(t *testing.T) {
	target := fixture(t)
	previous := helperSHA256
	helperSHA256 = "" // No helper can run in this test.
	t.Cleanup(func() { helperSHA256 = previous })
	calls := 0
	check := func(path string, pid int) error {
		calls++
		if path != target.ExecutablePath || pid != 0 {
			t.Fatal("checked a different core")
		}
		return nil
	}
	for range 3 {
		if err := authorize(context.Background(), target, check); err != nil {
			t.Fatal("persisted privileges requested authorization", err)
		}
	}
	if calls != 3 {
		t.Fatal("permission state was not rechecked on every launch")
	}
	err := authorize(context.Background(), target, func(string, int) error { return failure("linux_core_privileges_blocked") })
	if requirements.Code(err, "") != "linux_core_privileges_blocked" {
		t.Fatal("environment failure attempted helper authorization", err)
	}
}

func TestHelperMustMatchEmbeddedReleaseDigest(t *testing.T) {
	verifyHelper := func(ctx context.Context, path string) error {
		file, err := prepareHelper(ctx, path)
		if err != nil {
			return err
		}
		return file.Close()
	}
	target := fixture(t)
	previous := helperSHA256
	helperSHA256 = target.SHA256
	t.Cleanup(func() { helperSHA256 = previous })
	for _, mode := range []os.FileMode{0o700, 0o777, 0o644} {
		if err := os.Chmod(target.ExecutablePath, mode); err != nil {
			t.Fatal(err)
		}
		if err := verifyHelper(context.Background(), target.ExecutablePath); err != nil {
			t.Fatalf("matching helper with portable source mode %o: %v", mode, err)
		}
	}
	helperSHA256 = strings.Repeat("0", 64)
	if err := verifyHelper(context.Background(), target.ExecutablePath); err == nil {
		t.Fatal("mismatched helper accepted")
	}
	helperSHA256 = ""
	if err := verifyHelper(context.Background(), target.ExecutablePath); err == nil {
		t.Fatal("unconfigured development helper accepted")
	}
	helperSHA256 = target.SHA256
	link := filepath.Join(t.TempDir(), HelperName)
	if err := os.Symlink(target.ExecutablePath, link); err != nil {
		t.Fatal(err)
	}
	if err := verifyHelper(context.Background(), link); err == nil {
		t.Fatal("linked source accepted")
	}
	if err := os.Chmod(target.ExecutablePath, 0o777); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(target.ExecutablePath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteAt([]byte("changed"), 64)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyHelper(context.Background(), target.ExecutablePath); err == nil {
		t.Fatal("modified portable helper accepted")
	}
}

func TestPreparedHelperCannotChangeDuringAuthentication(t *testing.T) {
	target := fixture(t)
	previous := helperSHA256
	helperSHA256 = target.SHA256
	t.Cleanup(func() { helperSHA256 = previous })
	sealed, err := prepareHelper(context.Background(), target.ExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	if err := os.WriteFile(target.ExecutablePath, []byte("replaced on disk"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := verifyCore(context.Background(), sealed, target); err != nil {
		t.Fatal("disk replacement changed authorization executable", err)
	}
	if _, err := sealed.WriteAt([]byte("x"), 0); !errors.Is(err, unix.EPERM) {
		t.Fatal("memory executable remained writable", err)
	}
	if err := sealed.Truncate(0); !errors.Is(err, unix.EPERM) {
		t.Fatal("memory executable remained resizable", err)
	}
}
