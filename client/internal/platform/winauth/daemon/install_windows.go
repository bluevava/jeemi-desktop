//go:build windows

package daemon

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"io"
	"jeemi/internal/platform/helperstate"
	"os"
	"path/filepath"
	"time"
)

func stopService(ctx context.Context, service *mgr.Service) error {
	state, err := service.Query()
	if err != nil {
		return err
	}
	if state.State == svc.Stopped {
		return nil
	}
	oldProcess, openErr := windows.OpenProcess(windows.SYNCHRONIZE, false, state.ProcessId)
	if openErr != nil {
		return openErr
	}
	defer windows.CloseHandle(oldProcess)
	if _, err = service.Control(svc.Stop); err != nil && err != windows.ERROR_SERVICE_NOT_ACTIVE {
		return err
	}
	for {
		state, err = service.Query()
		if err != nil {
			return err
		}
		if state.State == svc.Stopped {
			wait, e := windows.WaitForSingleObject(oldProcess, 0)
			if e != nil {
				return e
			}
			if wait == windows.WAIT_OBJECT_0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Manage is the fixed administrator-approved installer, also usable when
// the old service cannot run. Its rollback never resumes a proxy automatically.
func Manage(sid string, remove bool, factory NetworkFactory) error {
	if !windows.GetCurrentProcessToken().IsElevated() || !validSID(sid) {
		return failure("authorization_identity_failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root, err := ensureStorage(sid)
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	service, err := m.OpenService(serviceName(sid))
	if err != nil && err != windows.ERROR_SERVICE_DOES_NOT_EXIST {
		return err
	}
	oldService := service != nil
	defer func() {
		if service != nil {
			service.Close()
		}
	}()
	binary, _ := serviceBinary(sid)
	var oldConfig mgr.Config
	if service != nil {
		oldConfig, err = service.Config()
		if err != nil {
			return err
		}
		if !serviceCommandMatches(oldConfig.BinaryPathName, binary, sid) {
			return failure("authorization_install_conflict")
		}
	}
	network, err := factory(root, sid)
	if err != nil {
		return err
	}
	if closer, ok := network.(interface{ Close() }); ok {
		defer closer.Close()
	}
	stop := func() error {
		if service != nil {
			if err := stopService(ctx, service); err != nil {
				return err
			}
		}
		if network.Restore(ctx) != nil {
			return failure("authorization_recovery_failed")
		}
		return nil
	}
	if remove {
		if err = stop(); err != nil {
			return err
		}
		return removeRegistration(sid, service)
	}
	var staged, backup string
	defer func() {
		if staged != "" {
			os.Remove(staged)
		}
	}()
	move := func(from, to string) error {
		a, _ := windows.UTF16PtrFromString(from)
		b, _ := windows.UTF16PtrFromString(to)
		return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	}
	err = helperstate.Replace(func() error { var e error; staged, e = stageInstaller(root); return e }, stop, func() error {
		if e := os.Remove(filepath.Join(root, "cleanup-complete")); e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if _, e := os.Lstat(binary); e == nil {
			if trustedPath(binary, false) != nil {
				return failure("authorization_identity_failed")
			}
			file, e := os.CreateTemp(root, ".previous-*.exe")
			if e != nil {
				return e
			}
			name := file.Name()
			file.Close()
			os.Remove(name)
			if e = move(binary, name); e != nil {
				return e
			}
			backup = name
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if e := move(staged, binary); e != nil {
			return e
		}
		// CreateService supplies a default for ServiceType, UpdateConfig does
		// not. Set it explicitly so repair/update uses the same valid SCM type.
		config := mgr.Config{ServiceType: windows.SERVICE_WIN32_OWN_PROCESS, StartType: mgr.StartAutomatic, ErrorControl: mgr.ErrorNormal, DisplayName: "Jeemi Authorization Helper", Description: "Jeemi authorization; waits for an explicit proxy start.", BinaryPathName: serviceCommand(binary, sid), ServiceStartName: "LocalSystem"}
		var e error
		if service == nil {
			service, e = m.CreateService(serviceName(sid), binary, config, "--service", sid)
		} else {
			e = service.UpdateConfig(config)
		}
		if e != nil {
			return e
		}
		if e = service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: time.Second}, {Type: mgr.ServiceRestart, Delay: 3 * time.Second}, {Type: mgr.NoAction}}, 60); e != nil {
			return e
		}
		return service.Start()
	}, func() error {
		for {
			state, e := service.Query()
			if e != nil {
				return e
			}
			if state.State == svc.Running {
				return nil
			}
			if state.State == svc.Stopped {
				return failure("authorization_service_failed")
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}, func() error {
		recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if service != nil {
			if e := stopService(recovery, service); e != nil {
				return e
			}
		}
		if backup != "" {
			if e := move(backup, binary); e != nil {
				return e
			}
		}
		if oldService {
			if e := service.UpdateConfig(oldConfig); e != nil {
				return e
			}
			return service.Start()
		}
		if service != nil {
			return service.Delete()
		}
		return nil
	})
	if err == nil && backup != "" {
		os.Remove(backup)
	}
	return err
}

func stageInstaller(root string) (string, error) {
	sourcePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	source, err := lockFile(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || info.Size() > 512<<20 {
		return "", failure("authorization_identity_failed")
	}
	temporary, err := os.CreateTemp(root, ".install-*.exe")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			os.Remove(temporary.Name())
		}
	}()
	if _, err = io.CopyN(temporary, source, info.Size()); err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	owner, _ := windows.StringToSid("S-1-5-32-544")
	if err = windows.SetNamedSecurityInfo(temporary.Name(), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, owner, nil, nil, nil); err != nil {
		return "", err
	}
	if err = trustedPath(temporary.Name(), false); err != nil {
		return "", err
	}
	success = true
	return temporary.Name(), nil
}

func removeRegistration(sid string, service *mgr.Service) error {
	if service != nil {
		if err := service.Delete(); err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			return err
		}
	}
	binary, err := serviceBinary(sid)
	if err != nil {
		return err
	}
	// Windows allows deletion of a running image through a disposition marked
	// for reboot only as a fallback; keep a cleanup receipt for harmless cache.
	if err = os.Remove(binary); err != nil && !errors.Is(err, os.ErrNotExist) {
		root, _ := serviceDirectory(sid)
		placeholder, e := os.CreateTemp(root, "retired-*.exe")
		if e != nil {
			return e
		}
		retired := placeholder.Name()
		placeholder.Close()
		os.Remove(retired)
		from, _ := windows.UTF16PtrFromString(binary)
		to, _ := windows.UTF16PtrFromString(retired)
		if e = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); e != nil {
			return e
		}
		if e = windows.MoveFileEx(to, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); e != nil {
			return e
		}
	}
	root, _ := serviceDirectory(sid)
	receipt := filepath.Join(root, "cleanup-complete")
	if err = os.WriteFile(receipt, []byte("Jeemi authorization cleanup v1\n"), 0600); err != nil {
		return err
	}
	owner, _ := windows.StringToSid("S-1-5-32-544")
	return windows.SetNamedSecurityInfo(receipt, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, owner, nil, nil, nil)
}
