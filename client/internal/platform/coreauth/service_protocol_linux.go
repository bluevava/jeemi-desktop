//go:build linux

package coreauth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"
	"jeemi/internal/platform/helperstate"
	"jeemi/internal/platform/processmemory"
)

type serviceRequest struct {
	Protocol      int      `json:"protocol"`
	Operation     string   `json:"operation"`
	Target        *Target  `json:"target,omitempty"`
	Targets       []Target `json:"targets,omitempty"`
	MihomoPID     int      `json:"mihomoPID,omitempty"`
	WebViewMemory bool     `json:"webViewMemory,omitempty"`
}
type serviceReply struct {
	Protocol int                           `json:"protocol"`
	Code     string                        `json:"code"`
	Digest   string                        `json:"digest,omitempty"`
	Memory   *processmemory.HelperSnapshot `json:"memory,omitempty"`
}

func decodeServiceRequest(data []byte) (serviceRequest, error) {
	var request serviceRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var extra any
	if len(data) > 64<<10 || decoder.Decode(&request) != nil || decoder.Decode(&extra) != io.EOF || request.Protocol != helperstate.Protocol {
		return request, failure("authorization_invalid_request")
	}
	switch request.Operation {
	case "memory":
		if request.Target != nil || request.Targets != nil || request.MihomoPID < 0 || request.MihomoPID > 1<<31-1 {
			return request, failure("authorization_invalid_request")
		}
	case "status":
		if request.Target != nil || request.Targets != nil {
			return request, failure("authorization_invalid_request")
		}
	case "authorize":
		if request.Target == nil || request.Targets != nil {
			return request, failure("authorization_invalid_request")
		}
	case "remove":
		if request.Target != nil || request.Targets == nil || len(request.Targets) > 256 {
			return request, failure("authorization_invalid_request")
		}
	default:
		return request, failure("authorization_invalid_request")
	}
	if request.Operation != "memory" && (request.MihomoPID != 0 || request.WebViewMemory) {
		return request, failure("authorization_invalid_request")
	}
	return request, nil
}

func unixPeer(conn *net.UnixConn) (*unix.Ucred, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var peer *unix.Ucred
	var socketErr error
	err = raw.Control(func(fd uintptr) { peer, socketErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED) })
	if err != nil {
		return nil, err
	}
	return peer, socketErr
}

func serviceCall(ctx context.Context, request serviceRequest) (serviceReply, error) {
	return serviceCallFor(ctx, uint32(os.Getuid()), request)
}

func serviceCallFor(ctx context.Context, uid uint32, request serviceRequest) (serviceReply, error) {
	var reply serviceReply
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", serviceSocket(uid))
	if err != nil {
		return reply, failure("authorization_repair_required")
	}
	defer connection.Close()
	peer, err := unixPeer(connection.(*net.UnixConn))
	if err != nil || peer.Uid != 0 {
		return reply, failure("authorization_identity_failed")
	}
	deadline := time.Now().Add(22 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = connection.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	request.Protocol = helperstate.Protocol
	if err = json.NewEncoder(connection).Encode(request); err != nil {
		return reply, err
	}
	if err = json.NewDecoder(io.LimitReader(connection, 4096)).Decode(&reply); err != nil {
		return reply, failure("authorization_repair_required")
	}
	if reply.Protocol != helperstate.Protocol {
		return reply, failure("authorization_update_required")
	}
	if reply.Code != "ok" {
		return reply, failure(reply.Code)
	}
	return reply, nil
}
