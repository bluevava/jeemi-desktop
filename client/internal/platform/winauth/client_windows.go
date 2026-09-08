//go:build windows

package winauth

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"jeemi/internal/platform/coresnapshot"
	"jeemi/internal/platform/helperstate"
)

func queryService(sid string) (*mgr.Service, error) {
	handleManager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, err
	}
	defer windows.CloseServiceHandle(handleManager)
	name, _ := windows.UTF16PtrFromString(serviceName(sid))
	handle, err := windows.OpenService(handleManager, name, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return nil, err
	}
	return &mgr.Service{Name: serviceName(sid), Handle: handle}, nil
}

func Status(ctx context.Context) (helperstate.State, error) {
	sid, err := ownerSID()
	if err != nil {
		return helperstate.State{}, err
	}
	root, err := serviceDirectory(sid)
	if err != nil {
		return helperstate.State{}, err
	}
	facts := helperstate.Facts{}
	if _, err = os.Lstat(root); err == nil {
		facts.Present = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return helperstate.State{}, failure("authorization_inspection_failed")
	}
	service, err := queryService(sid)
	if err == windows.ERROR_SERVICE_DOES_NOT_EXIST {
		receipt := filepath.Join(root, "cleanup-complete")
		if trustedPath(root, true) == nil && trustedPath(receipt, false) == nil {
			data, e := os.ReadFile(receipt)
			if e == nil && string(data) == "Jeemi authorization cleanup v1\n" {
				facts.Present = false
			}
		}
		return helperstate.EvaluateProtocol(facts, helperSHA256, Protocol), nil
	}
	if err != nil {
		return helperstate.State{}, failure("authorization_inspection_failed")
	}
	defer service.Close()
	facts.Registered = true
	config, err := service.Config()
	if err != nil {
		return helperstate.State{}, failure("authorization_inspection_failed")
	}
	binary, _ := serviceBinary(sid)
	facts.Trusted = registrationMatches(config, binary, sid)
	if !facts.Trusted {
		return helperstate.State{Present: true, Code: "authorization_service_registration", Action: "repair"}, nil
	}
	query, err := service.Query()
	if err != nil {
		return helperstate.State{}, failure("authorization_inspection_failed")
	}
	if query.State != svc.Running {
		return helperstate.State{Present: true, Code: "authorization_service_stopped", Action: "repair"}, nil
	}
	reply, err := Call(ctx, Request{Operation: "status"})
	if err != nil {
		code := "authorization_service_unreachable"
		if reply.Code == "authorization_recovery_failed" {
			code = reply.Code
		}
		if reply.Protocol != 0 && reply.Protocol != Protocol {
			return helperstate.State{Present: true, Code: "authorization_update_required", Action: "update"}, nil
		}
		return helperstate.State{Present: true, Code: code, Action: "repair"}, nil
	}
	// SCM controls the service registration; the kernel ties this reply to its
	// running PID. Installer ACL checks stay at installation, not every GUI RPC.
	facts.Reachable, facts.Protocol, facts.Digest = true, reply.Protocol, reply.Digest
	return helperstate.EvaluateProtocol(facts, helperSHA256, Protocol), nil
}

func connect(ctx context.Context) (net.Conn, error) {
	sid, err := ownerSID()
	if err != nil {
		return nil, err
	}
	dialContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := winio.DialPipeContext(dialContext, pipeName(sid))
	if err != nil {
		return nil, failure("authorization_repair_required")
	}
	bad := func() (net.Conn, error) { conn.Close(); return nil, failure("authorization_identity_failed") }
	pipe, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return bad()
	}
	var pid uint32
	if windows.GetNamedPipeServerProcessId(windows.Handle(pipe.Fd()), &pid) != nil {
		return bad()
	}
	service, err := queryService(sid)
	if err != nil {
		return bad()
	}
	defer service.Close()
	state, err := service.Query()
	if err != nil || state.ProcessId != pid || state.State != svc.Running {
		return bad()
	}
	config, err := service.Config()
	expected, _ := serviceBinary(sid)
	if err != nil || !registrationMatches(config, expected, sid) {
		return bad()
	}
	// An ordinary GUI cannot reliably OpenProcess on SYSTEM. SCM's protected
	// LocalSystem registration plus the kernel pipe-server PID is sufficient.
	return conn, nil
}

func Call(ctx context.Context, request Request) (Reply, error) { return call(ctx, request, nil) }
func call(ctx context.Context, request Request, snapshot io.Reader) (Reply, error) {
	connection, err := connect(ctx)
	if err != nil {
		return Reply{}, err
	}
	defer connection.Close()
	return exchange(ctx, connection, request, snapshot)
}

func exchange(ctx context.Context, connection net.Conn, request Request, snapshot io.Reader) (Reply, error) {
	var reply Reply
	deadline := time.Now().Add(2 * time.Minute)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	_ = connection.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { connection.Close() })
	defer stop()
	request.Protocol = Protocol
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return reply, redacted(err)
	}
	reader := bufio.NewReader(connection)
	readReply := func() error {
		data, err := reader.ReadSlice('\n')
		if err != nil || len(data) > 4096 || json.Unmarshal(data, &reply) != nil {
			return failure("authorization_service_failed")
		}
		if reply.Protocol != Protocol {
			return failure("authorization_update_required")
		}
		if reply.Code != "ok" && reply.Code != "snapshot" {
			return failure(reply.Code)
		}
		return nil
	}
	if snapshot != nil {
		if err := readReply(); err != nil {
			return reply, err
		}
		if reply.Code != "snapshot" {
			return reply, failure("authorization_invalid_request")
		}
		if _, err := io.CopyN(connection, snapshot, request.SnapshotSize); err != nil {
			return reply, redacted(err)
		}
	}
	err := readReply()
	if ctx.Err() != nil {
		return reply, ctx.Err()
	}
	if err == nil && reply.Code != "ok" {
		return reply, failure("authorization_invalid_request")
	}
	return reply, err
}

func SnapshotCall(ctx context.Context, request Request, home, configuration string, initial bool) (Reply, error) {
	relative, err := filepath.Rel(home, configuration)
	if err != nil {
		return Reply{}, failure("authorization_invalid_request")
	}
	request.Configuration = filepath.ToSlash(relative)
	request.SourceRoot = home
	if !coresnapshot.ConfigurationName(request.Configuration) {
		return Reply{}, failure("authorization_invalid_request")
	}
	archive, err := os.CreateTemp("", "jeemi-authorization-*.tar")
	if err != nil {
		return Reply{}, err
	}
	defer archive.Close()
	defer os.Remove(archive.Name())
	if err = coresnapshot.WriteSnapshot(home, request.Configuration, initial, archive); err != nil {
		return Reply{}, err
	}
	info, err := archive.Stat()
	if err != nil {
		return Reply{}, err
	}
	request.SnapshotSize = info.Size()
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return Reply{}, err
	}
	return call(ctx, request, archive)
}

func TargetForExecutable(executable string) (Target, error) {
	var target Target
	if filepath.Base(executable) != "mihomo.exe" || !filepath.IsAbs(executable) {
		return target, failure("authorization_core_unverified")
	}
	file, err := os.Open(filepath.Join(filepath.Dir(executable), "metadata.json"))
	if err != nil {
		return target, failure("authorization_core_unverified")
	}
	defer file.Close()
	var metadata struct {
		Version string `json:"version"`
		SHA256  string `json:"binarySha256"`
		Size    int64  `json:"binarySize"`
	}
	if json.NewDecoder(io.LimitReader(file, 65536)).Decode(&metadata) != nil {
		return target, failure("authorization_core_unverified")
	}
	target = Target{Version: metadata.Version, SHA256: metadata.SHA256, Size: metadata.Size, ExecutablePath: filepath.Clean(executable)}
	if !target.valid() || !managedCorePath(target) {
		return target, failure("authorization_core_unverified")
	}
	return target, nil
}
