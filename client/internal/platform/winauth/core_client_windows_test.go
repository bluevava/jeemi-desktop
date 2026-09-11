//go:build windows

package winauth

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"jeemi/internal/platform/helperstate"
)

func TestCoreCheckExchangesOnlyMetadataAndReportsOldHelperAsUpdate(t *testing.T) {
	_, target := coreFixture()
	for _, protocol := range []int{Protocol, 6, 5, 4, 3, 2, helperstate.Protocol} {
		client, server := net.Pipe()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := make(chan error, 1)
		go func() {
			defer server.Close()
			line, err := bufio.NewReader(server).ReadBytes('\n')
			if err == nil {
				var request Request
				request, err = decodeRequest(line)
				if err == nil && (request.Protocol != Protocol || request.Operation != "check" || *request.Target != target) {
					err = failure("authorization_invalid_request")
				}
			}
			if err == nil {
				err = json.NewEncoder(server).Encode(Reply{Protocol: protocol, Code: "ok"})
			}
			done <- err
		}()
		_, err := exchange(ctx, client, Request{Operation: "check", Target: &target}, nil)
		client.Close()
		cancel()
		if protocol == Protocol && err != nil {
			t.Fatal(err)
		}
		if protocol != Protocol && (err == nil || err.Error() != "authorization_update_required") {
			t.Fatalf("old helper did not require update: %v", err)
		}
		if err = <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestWindowsHelperStateUsesItsOwnProtocolVersion(t *testing.T) {
	_, target := coreFixture()
	facts := helperstate.Facts{Registered: true, Trusted: true, Reachable: true, Protocol: Protocol, Digest: target.SHA256}
	if state := helperstate.EvaluateProtocol(facts, target.SHA256, Protocol); !state.Ready {
		t.Fatalf("current Windows service rejected: %+v", state)
	}
	facts.Protocol = helperstate.Protocol
	if state := helperstate.EvaluateProtocol(facts, target.SHA256, Protocol); state.Ready || state.Action != "update" {
		t.Fatalf("old Windows service accepted: %+v", state)
	}
}
