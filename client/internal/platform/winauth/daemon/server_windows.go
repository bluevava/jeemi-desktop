//go:build windows

package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"jeemi/internal/platform/coresnapshot"
	"jeemi/internal/platform/processmemory"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/platform/winauth"
)

type server struct {
	preparation       preparationGate
	requests          chan struct{}
	recovery          bool
	sid, root, digest string
	network           Network
	cores             *serviceCores
	mu                sync.Mutex
	owner             windows.Handle
	ownerPID          uint32
	shutdown          chan struct{}
	once              sync.Once
}

func Run(sid string, factory NetworkFactory) error {
	if !validSID(sid) {
		return failure("authorization_identity_failed")
	}
	identity, err := ownerSID()
	if err != nil || identity != "S-1-5-18" {
		return failure("authorization_identity_failed")
	}
	root, err := ensureStorage(sid)
	if err != nil {
		return err
	}
	binary, _ := serviceBinary(sid)
	if trustedPath(binary, false) != nil {
		return failure("authorization_identity_failed")
	}
	digest, err := fileDigest(binary)
	if err != nil {
		return err
	}
	network, err := factory(root, sid)
	if err != nil {
		return err
	}
	cores, err := newServiceCores(root)
	if err != nil {
		return err
	}
	s := &server{requests: make(chan struct{}, 8), sid: sid, root: root, digest: digest, network: network, cores: cores, shutdown: make(chan struct{})}
	if closer, ok := network.(interface{ Close() }); ok {
		defer closer.Close()
	}
	return svc.Run(serviceName(sid), s)
}

func (s *server) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending, WaitHint: 15000}
	listener, err := winio.ListenPipe(pipeName(s.sid), &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GRGW;;;" + s.sid + ")", InputBufferSize: 65536, OutputBufferSize: 65536})
	if err != nil {
		return true, 1
	}
	defer listener.Close()
	// Always restore an interrupted proxy before accepting a new generation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = s.network.Restore(ctx)
	cancel()
	if err != nil {
		return true, 2
	}
	go s.serve(listener)
	go s.watchOwner()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case <-s.shutdown:
			status <- svc.Status{State: svc.StopPending, WaitHint: 10000}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				status <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending, WaitHint: 10000}
				prepareCtx, cancelPrepare := context.WithTimeout(context.Background(), 2*time.Second)
				prepareErr := s.preparation.close(prepareCtx)
				cancelPrepare()
				s.mu.Lock()
				err := s.stop()
				s.mu.Unlock()
				s.once.Do(func() { close(s.shutdown) })
				if err != nil || prepareErr != nil {
					return true, 2
				}
				return false, 0
			}
		}
	}
}

func (s *server) serve(listener net.Listener) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		select {
		case s.requests <- struct{}{}:
			go func() { defer func() { <-s.requests }(); s.handle(connection) }()
		default:
			connection.Close()
		}
	}
}
func (s *server) handle(connection net.Conn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Minute))
	pipe, ok := connection.(interface{ Fd() uintptr })
	if !ok {
		return
	}
	var pid uint32
	if windows.GetNamedPipeClientProcessId(windows.Handle(pipe.Fd()), &pid) != nil {
		return
	}
	caller, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(caller)
	identity, err := processHandleSID(caller)
	if err != nil || identity != s.sid {
		return
	}
	reader := bufio.NewReaderSize(connection, maxRequestBytes+1)
	data, err := reader.ReadSlice('\n')
	if err != nil {
		return
	}
	request, err := decodeRequest(data)
	write := func(reply Reply) error {
		reply.Protocol = Protocol
		return json.NewEncoder(connection).Encode(reply)
	}
	if err != nil {
		write(Reply{Code: "authorization_invalid_request"})
		return
	}
	if request.Protocol != Protocol {
		write(Reply{Code: "authorization_update_required"})
		return
	}
	if request.Operation == "check" || request.Operation == "prepare" {
		// File work does not hold the network/lifecycle mutex.
		coreErr := s.prepareCore(caller, *request.Target)
		code := "ok"
		if coreErr != nil {
			code = requirements.Code(coreErr, "authorization_core_unverified")
		}
		write(Reply{Code: code})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.shutdown:
		write(Reply{Code: "authorization_repair_required"})
		return
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	reply := Reply{Code: "ok", Digest: s.digest}
	remove := false
	switch request.Operation {
	case "status":
		if s.recovery {
			err = failure("authorization_recovery_failed")
		}
	case "state":
		reply.PID, reply.Running = s.cores.state()

	case "memory":
		if s.ownerPID != 0 && s.ownerPID != pid {
			err = failure("authorization_busy")
			break
		}
		corePID, running := s.cores.state()
		if !running {
			corePID = 0
		}
		memory := processmemory.CaptureHelper(corePID)
		if current, alive := s.cores.state(); corePID > 0 && (!alive || current != corePID) {
			memory.MihomoBytes = nil
		}
		if request.WebViewMemory {
			memory.WebViewBytes = processmemory.CaptureWebView(int(pid), corePID)
		}
		reply.Memory = &memory

	case "start":
		if wait, e := windows.WaitForSingleObject(caller, 0); e != nil || wait != uint32(windows.WAIT_TIMEOUT) {
			err = failure("authorization_identity_failed")
			break
		}
		if notifier, ok := s.network.(interface{ BindClient(uint32) error }); ok {
			if err = notifier.BindClient(pid); err != nil {
				break
			}
		}
		if err = s.network.Restore(ctx); err != nil {
			s.recovery = true
			break
		}
		s.recovery = false
		if _, running := s.cores.state(); running {
			err = failure("authorization_busy")
			break
		}
		// A kernel process handle binds the lifetime, avoiding PID reuse.
		var owner windows.Handle
		err = windows.DuplicateHandle(windows.CurrentProcess(), caller, windows.CurrentProcess(), &owner, 0, false, windows.DUPLICATE_SAME_ACCESS)
		if err != nil {
			break
		}
		if s.owner != 0 {
			windows.CloseHandle(s.owner)
		}
		s.owner = owner
		s.ownerPID = pid
		var workspace string
		workspace, err = privateSession(s.root)
		if err == nil {
			if err = write(Reply{Code: "snapshot"}); err == nil {
				err = s.cores.snapshot(workspace, request, reader, true)
			}
		}
		if err == nil {
			var binary *winauth.LockedCore
			binary, err = openCallerCore(ctx, caller, *request.Target)
			if err == nil {
				err = s.cores.start(binary, workspace, request.Configuration)
			}
		}
		if err != nil {
			if workspace != "" {
				_ = removeWorkspace(s.root, workspace)
			}
			windows.CloseHandle(s.owner)
			s.owner = 0
			s.ownerPID = 0
		} else {
			reply.PID, reply.Running = s.cores.state()
		}
	case "stage":
		if s.ownerPID != pid || s.cores.workspace == "" {
			err = failure("authorization_identity_failed")
			break
		}
		if err = write(Reply{Code: "snapshot"}); err == nil {
			err = s.cores.snapshot(s.cores.workspace, request, reader, false)
		}
		if err == nil {
			reply.Configuration = filepath.Join(s.cores.workspace, filepath.FromSlash(request.Configuration))
		}
	case "proxy":
		if s.ownerPID != pid {
			err = failure("authorization_identity_failed")
			break
		}
		_, running := s.cores.state()
		if !running {
			err = failure("authorization_core_failed")
			break
		}
		if !ownsTCP(uint32(s.cores.command.Process.Pid), uint16(request.Preferences.ListenPort), false) {
			err = failure("authorization_core_failed")
			break
		}
		err = s.network.Apply(ctx, *request.Preferences)
	case "restore":
		err = s.network.Restore(ctx)
		s.recovery = err != nil
	case "stop":
		err = s.stop()
	case "remove":
		defer s.preparation.resume()
		if err = s.preparation.close(ctx); err != nil {
			break
		}
		err = s.stop()
		if err != nil {
			break
		}
		var manager *mgr.Mgr
		manager, err = mgr.Connect()
		if err != nil {
			break
		}
		defer manager.Disconnect()
		var service *mgr.Service
		service, err = manager.OpenService(serviceName(s.sid))
		if err != nil {
			break
		}
		defer service.Close()
		err = removeRegistration(s.sid, service)
		remove = err == nil
	}
	if err != nil {
		reply.Code = requirements.Code(err, "authorization_service_failed")
	}
	if write(reply) != nil && request.Operation == "start" {
		_ = s.stop()
	}
	if remove {
		s.once.Do(func() { close(s.shutdown) })
	}
}

func (s *server) stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err := s.network.Restore(ctx)
	// Even if proxy restoration fails, stop the process tree; retain its
	// recovery record and refuse uninstall so service restart can retry.
	coreErr := s.cores.stop(ctx)
	s.recovery = err != nil || coreErr != nil
	if s.owner != 0 {
		windows.CloseHandle(s.owner)
		s.owner = 0
		s.ownerPID = 0
	}
	if err != nil {
		return failure("authorization_recovery_failed")
	}
	return coreErr
}
func (s *server) watchOwner() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.recovery {
				_ = s.stop()
			}
			if s.owner != 0 {
				wait, err := windows.WaitForSingleObject(s.owner, 0)
				_, running := s.cores.state()
				if err != nil || wait == windows.WAIT_OBJECT_0 || !running {
					_ = s.stop()
				}
			}
			s.mu.Unlock()
		}
	}
}

func removeWorkspace(root, workspace string) error {
	relative, err := filepath.Rel(root, workspace)
	if err != nil || filepath.Dir(relative) != "." || len(relative) < 9 || relative[:8] != "session-" {
		return failure("authorization_snapshot_unsafe_path")
	}
	return os.RemoveAll(workspace)
}

func (c *serviceCores) snapshot(directory string, request Request, input io.Reader, initial bool) error {
	if !coresnapshot.ConfigurationName(request.Configuration) {
		return failure("authorization_invalid_request")
	}
	if initial {
		c.digests = map[string]string{}
	}
	limited := &io.LimitedReader{R: input, N: request.SnapshotSize}
	if err := coresnapshot.Receive(directory, request.SourceRoot, limited, c.digests); err != nil {
		return err
	}
	_, err := io.Copy(io.Discard, limited)
	if err != nil || limited.N != 0 {
		return failure("authorization_snapshot_unsafe_path")
	}
	return nil
}
