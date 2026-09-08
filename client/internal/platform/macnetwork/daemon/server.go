package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/platform/processmemory"
)

type Process interface {
	PID() int
	Running() bool
	Stop() error
}
type CoreBackend interface {
	Prepare(context.Context, macnetwork.Target) error
	Check(macnetwork.Target) error
	Start(uint32, macnetwork.Target, string) (Process, error)
	Device() (string, error)
	VerifyProxy(string) error
	VerifyDNS(string) error
}
type DNSGateway interface{ Close() error }
type Server struct {
	mu             sync.Mutex
	preparationMu  sync.Mutex
	preparingOwner uint64
	prepareCancel  context.CancelFunc
	cores          CoreBackend
	journal        *Journal
	openDNS        func(string) (DNSGateway, string, error)
	dns            DNSGateway
	owner          uint64
	process        Process
	device         string
	failedRecovery bool
	closed         bool
	clients        map[uint64]bool
	memory         func(int) processmemory.HelperSnapshot
	webMemory      func(int, int) *uint64
}

func NewServer(cores CoreBackend, journal *Journal, openDNS func(string) (DNSGateway, string, error)) *Server {
	server := &Server{cores: cores, journal: journal, openDNS: openDNS, clients: map[uint64]bool{}, memory: processmemory.CaptureHelper, webMemory: processmemory.CaptureWebView}
	server.failedRecovery = journal.Restore() != nil
	return server
}

func decode(data []byte) (macnetwork.Request, error) {
	var req macnetwork.Request
	if len(data) > macnetwork.MaxMessageBytes {
		return req, macnetwork.Failure("invalid_request")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || req.Protocol != macnetwork.ProtocolVersion {
		return req, macnetwork.Failure("invalid_request")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return req, macnetwork.Failure("invalid_request")
	}
	if req.WebViewMemory && req.Operation != "memory" || req.Operation == "memory" && (req.Target != nil || req.Configuration != "" || req.Endpoint != "" || req.DNS != "") {
		return req, macnetwork.Failure("invalid_request")
	}
	return req, nil
}

func (s *Server) Handle(client uint64, uid uint32, clientPID int, data []byte) []byte {
	reply, err := s.handle(client, uid, clientPID, data)
	if err != nil {
		reply.Code = macnetwork.ErrorCode(err)
	} else {
		reply.Code = "ok"
	}
	return reply.Marshal()
}

func (s *Server) handle(client uint64, uid uint32, clientPID int, data []byte) (macnetwork.Response, error) {
	if client == 0 || uid < 500 {
		return macnetwork.Response{}, macnetwork.Failure("invalid_request")
	}
	req, err := decode(data)
	if err != nil {
		return macnetwork.Response{}, err
	}
	if req.Operation == "cancel_prepare" {
		s.preparationMu.Lock()
		if s.preparingOwner == client && s.prepareCancel != nil {
			s.prepareCancel()
		}
		s.preparationMu.Unlock()
		return macnetwork.Response{}, nil
	}
	if req.Operation == "prepare" {
		if req.Target == nil || req.Target.Validate() != nil {
			return macnetwork.Response{}, macnetwork.Failure("core_unverified")
		}
		s.mu.Lock()
		blocked := !s.clients[client] || s.closed || s.failedRecovery || (s.owner != 0 && s.owner != client)
		s.mu.Unlock()
		if blocked {
			return macnetwork.Response{}, macnetwork.Failure("busy")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
		defer cancel()
		s.preparationMu.Lock()
		if s.prepareCancel != nil {
			s.preparationMu.Unlock()
			return macnetwork.Response{}, macnetwork.Failure("busy")
		}
		s.preparingOwner = client
		s.prepareCancel = cancel
		s.preparationMu.Unlock()
		defer func() { s.preparationMu.Lock(); s.preparingOwner = 0; s.prepareCancel = nil; s.preparationMu.Unlock() }()
		err := s.cores.Prepare(ctx, *req.Target)
		if ctx.Err() != nil {
			return macnetwork.Response{}, macnetwork.Failure("cancelled")
		}
		return macnetwork.Response{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.clients[client] {
		return macnetwork.Response{}, macnetwork.Failure("unreachable")
	}
	if req.Operation == "ping" {
		if s.failedRecovery {
			return macnetwork.Response{}, macnetwork.Failure("recovery_failed")
		}
		return macnetwork.Response{}, nil
	}
	if s.owner != 0 && s.owner != client {
		return macnetwork.Response{}, macnetwork.Failure("busy")
	}
	if s.failedRecovery && req.Operation != "restore" && req.Operation != "stop" && req.Operation != "memory" {
		return macnetwork.Response{}, macnetwork.Failure("recovery_failed")
	}
	switch req.Operation {
	case "memory":
		pid := 0
		if s.process != nil && s.process.Running() {
			pid = s.process.PID()
		}
		memory := s.memory(pid)
		if pid > 0 && !s.process.Running() {
			memory.MihomoBytes = nil
		}
		if req.WebViewMemory && clientPID > 0 {
			memory.WebViewBytes = s.webMemory(clientPID, pid)
		}
		return macnetwork.Response{Memory: &memory}, nil
	case "check":
		if req.Target == nil {
			return macnetwork.Response{}, macnetwork.Failure("core_unverified")
		}
		return macnetwork.Response{}, s.cores.Check(*req.Target)
	case "start":
		if s.process != nil && s.process.Running() {
			return macnetwork.Response{}, macnetwork.Failure("busy")
		}
		if req.Target == nil {
			return macnetwork.Response{}, macnetwork.Failure("core_unverified")
		}
		if err := s.restore(); err != nil {
			return macnetwork.Response{}, err
		}
		process, err := s.cores.Start(uid, *req.Target, req.Configuration)
		if err != nil {
			return macnetwork.Response{}, err
		}
		s.owner = client
		s.process = process
		return macnetwork.Response{PID: process.PID(), Running: true}, nil
	case "stage":
		if s.process == nil || !s.process.Running() {
			return macnetwork.Response{}, macnetwork.Failure("core_failed")
		}
		backend, ok := s.cores.(interface{ Stage(string) (string, error) })
		if !ok {
			return macnetwork.Response{}, macnetwork.Failure("invalid_request")
		}
		path, err := backend.Stage(req.Configuration)
		return macnetwork.Response{Configuration: path}, err
	case "state":
		if s.process == nil {
			return macnetwork.Response{}, nil
		}
		return macnetwork.Response{PID: s.process.PID(), Running: s.process.Running(), Device: s.device}, nil
	case "stop":
		return macnetwork.Response{}, s.stop()
	case "restore":
		return macnetwork.Response{}, s.restore()
	case "proxy":
		if s.process == nil || !s.process.Running() {
			return macnetwork.Response{}, macnetwork.Failure("core_failed")
		}
		if err := s.cores.VerifyProxy(req.Endpoint); err != nil {
			return macnetwork.Response{}, err
		}
		if err := s.journal.ApplyProxy(req.Endpoint); err != nil {
			_ = s.restore()
			return macnetwork.Response{}, macnetwork.Failure("network_failed")
		}
		return macnetwork.Response{}, nil
	case "tun":
		if s.process == nil || !s.process.Running() {
			return macnetwork.Response{}, macnetwork.Failure("core_failed")
		}
		device, err := s.cores.Device()
		if err != nil {
			return macnetwork.Response{}, err
		}
		if err = s.restore(); err != nil {
			return macnetwork.Response{}, err
		}
		if req.DNS != "" {
			if err := s.cores.VerifyDNS(req.DNS); err != nil {
				return macnetwork.Response{}, err
			}
			gateway, address, err := s.openDNS(req.DNS)
			if err != nil {
				return macnetwork.Response{}, err
			}
			s.dns = gateway
			if err = s.journal.ApplyDNS(address); err != nil {
				_ = s.restore()
				return macnetwork.Response{}, macnetwork.Failure("network_failed")
			}
		}
		s.device = device
		return macnetwork.Response{Device: device}, nil
	default:
		return macnetwork.Response{}, macnetwork.Failure("invalid_request")
	}
}

func (s *Server) restore() error {
	err := s.journal.Restore()
	// Keep the DNS relay alive while restoration is failing and the core is
	// still healthy; retain the durable journal for a later retry.
	if err != nil {
		s.failedRecovery = true
		return macnetwork.Failure("recovery_failed")
	}
	if s.dns != nil {
		_ = s.dns.Close()
		s.dns = nil
	}
	s.device = ""
	s.failedRecovery = false
	return nil
}

func (s *Server) stop() error {
	err := s.restore()
	if s.process != nil {
		if stopErr := s.process.Stop(); err == nil {
			err = stopErr
		}
		s.process = nil
	}
	if s.dns != nil {
		_ = s.dns.Close()
		s.dns = nil
	}
	s.owner = 0
	return err
}

func (s *Server) Connected(client uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.clients[client] = true
	}
}

func (s *Server) Disconnected(client uint64) {
	s.preparationMu.Lock()
	if s.preparingOwner == client && s.prepareCancel != nil {
		s.prepareCancel()
	}
	s.preparationMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
	if s.owner == client {
		_ = s.stop()
	}
}

// Tick covers interface/location changes and an exited core even when the GUI
// is gone. It never launches a core or requests new authorization.
func (s *Server) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.failedRecovery {
		_ = s.restore()
		return
	}
	if s.process != nil && !s.process.Running() {
		_ = s.stop()
		return
	}
	if err := s.journal.Refresh(); err != nil {
		_ = s.stop()
	}
}
func (s *Server) Close() {
	s.preparationMu.Lock()
	if s.prepareCancel != nil {
		s.prepareCancel()
	}
	s.preparationMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	_ = s.stop()
}
