//go:build linux

package coreauth

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"jeemi/internal/platform/helperstate"
	"jeemi/internal/platform/requirements"
)

// RunService grants only the fixed capabilities and resolver policy. It never
// launches a root core or accepts arbitrary commands, unit names or identities.
func RunService(value string) int {
	uid, err := serviceUID(value)
	if err != nil || os.Getuid() != 0 || os.Geteuid() != 0 {
		return 1
	}
	digest, err := installedServiceDigest(uid)
	if err != nil {
		return 1
	}
	executable, err := os.Executable()
	if err != nil || executable != serviceBinary(uid) {
		return 1
	}
	directory, err := openPolicyDirectory(filepath.Dir(serviceSocket(uid)), 0)
	if err != nil {
		return 1
	}
	directory.Close()
	if info, err := os.Lstat(serviceSocket(uid)); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return 1
		}
		if os.Remove(serviceSocket(uid)) != nil {
			return 1
		}
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: serviceSocket(uid), Net: "unix"})
	if err != nil {
		return 1
	}
	defer listener.Close()
	if os.Chown(serviceSocket(uid), int(uid), -1) != nil || os.Chmod(serviceSocket(uid), 0600) != nil {
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	go func() { <-ctx.Done(); _ = listener.Close() }()
	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			if ctx.Err() != nil {
				return 0
			}
			return 1
		}
		peer, err := unixPeer(connection)
		if err != nil || (peer.Uid != uid && peer.Uid != 0) {
			connection.Close()
			continue
		}
		_ = connection.SetDeadline(time.Now().Add(22 * time.Second))
		reader := bufio.NewReaderSize(connection, 64<<10+1)
		data, err := reader.ReadSlice('\n')
		request, decodeErr := decodeServiceRequest(data)
		reply := serviceReply{Protocol: helperstate.Protocol, Digest: digest}
		remove := false
		if err == nil && decodeErr == nil && (peer.Uid == uid || request.Operation == "status") {
			operation, cancelOp := context.WithTimeout(ctx, 20*time.Second)
			operation, cancelLease := signal.NotifyContext(operation, syscall.SIGIO)
			switch request.Operation {
			case "memory":
				memory := serviceMemory(int(peer.Pid), uid, request.MihomoPID, request.WebViewMemory)
				reply.Memory = &memory
			case "authorize":
				err = grant(operation, *request.Target, uid, kernelGrantOps{})
				if err == nil {
					err = authorizeResolver(operation, uid, int(peer.Pid))
				}
			case "remove":
				err = cleanupForUID(operation, request.Targets, uid)
				if err == nil {
					err = removeService(operation, uid, false)
					remove = err == nil
				}
			}
			cancelLease()
			cancelOp()
		} else {
			err = failure("authorization_invalid_request")
		}
		code := "ok"
		if err != nil {
			code = requirements.Code(err, "authorization_service_failed")
		}
		reply.Code = code
		_ = json.NewEncoder(connection).Encode(reply)
		connection.Close()
		if remove {
			return 0
		}
	}
}

func cleanupForUID(ctx context.Context, targets []Target, uid uint32) error {
	for _, target := range targets {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := revoke(target, uid, kernelGrantOps{}); err != nil {
			return err
		}
	}
	policy, err := resolverPolicyFor(uid)
	if err != nil {
		return err
	}
	return policy.remove(resolverRulesDirectory, 0)
}
