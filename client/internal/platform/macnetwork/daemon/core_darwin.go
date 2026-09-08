//go:build darwin

package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"jeemi/internal/core"
	"jeemi/internal/platform/macnetwork"
)

type protectedCores struct {
	manager   *core.VersionManager
	pid       int
	workspace string
	uid       uint32
	digests   map[string]string
}

func (c *protectedCores) Prepare(ctx context.Context, target macnetwork.Target) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if _, err := c.find(target); err == nil {
		return nil
	}
	// Independently retrieve and verify the official release into root-owned
	// storage. Never execute a GUI/user-writable import as root.
	if _, err := c.manager.Download(ctx, target.Version); err != nil {
		return macnetwork.Failure("core_unverified")
	}
	_, err := c.find(target)
	return err
}
func (c *protectedCores) Check(target macnetwork.Target) error { _, err := c.find(target); return err }
func (c *protectedCores) find(target macnetwork.Target) (string, error) {
	if err := target.Validate(); err != nil {
		return "", err
	}
	state, err := c.manager.State()
	if err != nil {
		return "", macnetwork.Failure("integrity_failed")
	}
	for _, installed := range state.InstalledVersions {
		if installed.Version != target.Version {
			continue
		}
		if installed.Source != core.InstallationSourceOfficial || installed.BinarySHA256 != target.SHA256 || installed.BinarySize != target.Size {
			return "", macnetwork.Failure("integrity_failed")
		}
		if err := rootOwned(installed.ExecutablePath, false); err != nil {
			return "", err
		}
		return installed.ExecutablePath, nil
	}
	return "", macnetwork.Failure("core_missing")
}

func (c *protectedCores) Start(uid uint32, target macnetwork.Target, relative string) (Process, error) {
	binary, err := c.find(target)
	if err != nil {
		return nil, err
	}
	if !configurationName(relative) {
		return nil, macnetwork.Failure("unsafe_path")
	}
	workspace, err := os.MkdirTemp(rootDirectory, "session-")
	if err != nil {
		return nil, macnetwork.Failure("unsafe_path")
	}
	started := false
	defer func() {
		if !started {
			removeWorkspace(workspace)
		}
	}()
	digests := map[string]string{}
	if err = snapshotAsUser(uid, relative, workspace, true, digests); err != nil {
		return nil, err
	}
	configuration := filepath.Join(workspace, filepath.FromSlash(relative))
	if info, err := os.Lstat(configuration); err != nil || !info.Mode().IsRegular() {
		return nil, macnetwork.Failure("unsafe_path")
	}
	runtimeHome := workspace
	data := workspace
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		return nil, macnetwork.Failure("sandbox_unavailable")
	}
	profile, err := SandboxProfile(data, runtimeHome, binary)
	if err != nil {
		return nil, err
	}
	command := exec.Command("/usr/bin/sandbox-exec", "-p", profile, binary, "-d", runtimeHome, "-f", configuration)
	command.Dir = runtimeHome
	command.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + data, "TMPDIR=" + runtimeHome, "LANG=en_US.UTF-8"}
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	// Preserve launchd's process group. AbandonProcessGroup=false makes launchd
	// reap the core if the privileged helper crashes; do not use Setpgid here.
	if err = command.Start(); err != nil {
		return nil, macnetwork.Failure("core_failed")
	}
	started = true
	c.pid = command.Process.Pid
	c.workspace = workspace
	c.uid = uid
	c.digests = digests
	p := &helperProcess{command: command, done: make(chan struct{}), workspace: workspace}
	go func() { _ = command.Wait(); close(p.done) }()
	return p, nil
}

func (c *protectedCores) Device() (string, error) {
	device, _, err := macnetwork.ProcessNetwork(c.pid, "", 0)
	if err != nil || device == "" {
		return "", macnetwork.Failure("tun_unavailable")
	}
	info, err := net.InterfaceByName(device)
	if err != nil || info.Flags&net.FlagUp == 0 {
		return "", macnetwork.Failure("tun_unavailable")
	}
	addresses, err := info.Addrs()
	if err != nil || len(addresses) == 0 {
		return "", macnetwork.Failure("tun_unavailable")
	}
	return device, nil
}
func (c *protectedCores) VerifyDNS(address string) error {
	host, port, err := net.SplitHostPort(address)
	number, parseErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || parseErr != nil || number < 1 || number > 65535 || ip == nil || (!ip.IsLoopback() && !ip.IsUnspecified()) {
		return macnetwork.Failure("dns_invalid")
	}
	if ip.IsUnspecified() {
		host = "127.0.0.1"
		if ip.To4() == nil {
			host = "::1"
		}
	}
	_, ready, err := macnetwork.ProcessNetwork(c.pid, host, number)
	if err != nil || !ready {
		return macnetwork.Failure("dns_invalid")
	}
	return nil
}

type helperProcess struct {
	command   *exec.Cmd
	done      chan struct{}
	workspace string
}

func (p *helperProcess) PID() int { return p.command.Process.Pid }
func (p *helperProcess) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func (p *helperProcess) Stop() error {
	if p.Running() {
		_ = p.command.Process.Signal(syscall.SIGTERM)
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			_ = p.command.Process.Kill()
			select {
			case <-p.done:
			case <-time.After(2 * time.Second):
				return macnetwork.Failure("core_failed")
			}
		}
	}
	removeWorkspace(p.workspace)
	return nil
}

func (c *protectedCores) Stage(configuration string) (string, error) {
	if c.workspace == "" || !configurationName(configuration) {
		return "", macnetwork.Failure("unsafe_path")
	}
	if err := snapshotAsUser(c.uid, configuration, c.workspace, false, c.digests); err != nil {
		return "", err
	}
	path := filepath.Join(c.workspace, filepath.FromSlash(configuration))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", macnetwork.Failure("unsafe_path")
	}
	return path, nil
}

func rootOwned(path string, directory bool) error {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path {
		return macnetwork.Failure("unsafe_path")
	}
	current := path
	for {
		info, err := os.Lstat(current)
		if err != nil || info.Mode().Perm()&0022 != 0 {
			return macnetwork.Failure("unsafe_path")
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 {
			return macnetwork.Failure("unsafe_path")
		}
		if current == path {
			if info.IsDir() != directory || (!directory && (!info.Mode().IsRegular() || stat.Nlink != 1)) {
				return macnetwork.Failure("unsafe_path")
			}
		} else if !info.IsDir() {
			return macnetwork.Failure("unsafe_path")
		}
		if current == "/" {
			break
		}
		current = filepath.Dir(current)
	}
	return nil
}

// Used by the real daemon only; tests inject a fake core backend.
func newProtectedCores(directory string) (*protectedCores, error) {
	client := &http.Client{Timeout: 110 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if request.URL.Scheme != "https" || len(via) >= 10 {
			return fmt.Errorf("invalid official download redirect")
		}
		return nil
	}}
	manager, err := core.NewVersionManager(core.VersionManagerOptions{DataDirectory: directory, HTTPClient: client})
	return &protectedCores{manager: manager}, err
}

func (c *protectedCores) VerifyProxy(endpoint string) error {
	values, err := ProxyValues(endpoint)
	if err != nil {
		return err
	}
	for _, prefix := range []string{"HTTP", "HTTPS", "SOCKS"} {
		if enabled, _ := values[prefix+"Enable"].(int); enabled != 1 {
			continue
		}
		port, ok := values[prefix+"Port"].(int)
		if !ok || !macnetwork.HasTCPListener(c.pid, "127.0.0.1", port) {
			return macnetwork.Failure("core_failed")
		}
	}
	return nil
}
