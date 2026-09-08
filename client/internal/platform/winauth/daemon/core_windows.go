//go:build windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"jeemi/internal/platform/winauth"

	"golang.org/x/sys/windows"
	"gopkg.in/yaml.v3"
)

type serviceCores struct {
	root                     string
	command                  *exec.Cmd
	job                      windows.Handle
	done                     chan struct{}
	workspace, configuration string
	digests                  map[string]string
}

func newServiceCores(root string) (*serviceCores, error) {
	if err := trustedPath(root, true); err != nil {
		return nil, err
	}
	return &serviceCores{root: root}, nil
}
func (c *serviceCores) state() (int, bool) {
	if c.command == nil {
		return 0, false
	}
	select {
	case <-c.done:
		return c.command.Process.Pid, false
	default:
		return c.command.Process.Pid, true
	}
}

// start owns the verified data-directory core and its path locks until Wait
// finishes, including startup failure and unexpected exits.
func (c *serviceCores) start(binary *winauth.LockedCore, workspace, configuration string) error {
	successful := false
	defer func() {
		if !successful {
			binary.Close()
		}
	}()
	if _, running := c.state(); running {
		return failure("authorization_busy")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return err
	}
	// The GUI and core share the generation's loopback controller session.
	// Authorization only supplies the process privileges and lifetime here.
	command := exec.Command(binary.Name(), "-d", workspace, "-f", filepath.Join(workspace, filepath.FromSlash(configuration)))
	command.Dir = workspace
	windowsDir, err := windows.GetWindowsDirectory()
	if err != nil {
		windows.CloseHandle(job)
		return err
	}
	command.Env = []string{"SystemRoot=" + windowsDir, "WINDIR=" + windowsDir, "PATH=" + filepath.Join(windowsDir, "System32"), "TEMP=" + workspace, "TMP=" + workspace, "USERPROFILE=" + workspace}
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	// Prevent a core from executing or spawning children before Job attachment.
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED}
	if err = command.Start(); err != nil {
		windows.CloseHandle(job)
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(command.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
	}
	if err == nil {
		status, _, _ := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess").Call(uintptr(process))
		if status != 0 {
			err = failure("authorization_core_failed")
		}
	}
	if process != 0 {
		windows.CloseHandle(process)
	}
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		windows.CloseHandle(job)
		return err
	}
	if c.job != 0 {
		windows.CloseHandle(c.job)
	}
	c.command, c.job, c.workspace, c.configuration = command, job, workspace, configuration
	successful = true
	c.done = make(chan struct{})
	done := c.done
	go func() { _ = command.Wait(); _ = binary.Close(); close(done) }()
	return nil
}
func (c *serviceCores) stop(ctx context.Context) error {
	if c.command == nil {
		return nil
	}
	if _, running := c.state(); running {
		// Give mihomo an authenticated chance to remove its TUN routes first.
		c.disableTUN(ctx)
		if c.job != 0 {
			_ = windows.TerminateJobObject(c.job, 1)
		}
		select {
		case <-c.done:
		case <-ctx.Done():
			return failure("authorization_core_failed")
		}
	}
	if c.job != 0 {
		windows.CloseHandle(c.job)
		c.job = 0
	}
	c.command = nil
	if c.workspace != "" {
		if err := removeWorkspace(c.root, c.workspace); err != nil {
			return err
		}
		c.workspace = ""
	}
	return nil
}
func (c *serviceCores) disableTUN(ctx context.Context) {
	data, err := os.ReadFile(filepath.Join(c.workspace, filepath.FromSlash(c.configuration)))
	if err != nil || len(data) > 1<<20 {
		return
	}
	var configuration struct {
		Controller string `yaml:"external-controller"`
		Secret     string `yaml:"secret"`
	}
	if yaml.Unmarshal(data, &configuration) != nil || !strings.HasPrefix(configuration.Controller, "127.0.0.1:") || len(configuration.Secret) < 32 {
		return
	}
	_, port, _ := net.SplitHostPort(configuration.Controller)
	number, _ := strconv.Atoi(port)
	if !ownsTCP(uint32(c.command.Process.Pid), uint16(number), true) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"tun": map[string]any{"enable": false}})
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, "http://"+configuration.Controller+"/configs", bytes.NewReader(body))
	if err != nil {
		return
	}
	request.Header.Set("Authorization", "Bearer "+configuration.Secret)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 2 * time.Second}
	if response, err := client.Do(request); err == nil {
		response.Body.Close()
	}
}
