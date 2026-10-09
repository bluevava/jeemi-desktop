package runtime

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/subscription"
)

func directDownloadManager(t *testing.T, address string) *Manager {
	t.Helper()
	m, err := NewManager(Options{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	m.active = &Generation{ID: "active", DNS: dnstool.Session{
		DirectAddress: address, Username: "jeemi-dns", Password: strings.Repeat("d", 64),
	}}
	m.status = Status{State: StateRunning, ControllerReady: true}
	m.client = newControllerClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		t.Error("direct download changed routing through the controller")
		return nil, context.Canceled
	})}, ControllerSession{Secret: "controller-secret-must-not-be-forwarded"})
	return m
}

func TestDirectDownloadUsesAuthenticatedMihomoRouteAndSubscriptionChecks(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	for _, tun := range []bool{false, true} {
		t.Run(map[bool]string{false: "system proxy", true: "TUN"}[tun], func(t *testing.T) {
			var requests atomic.Int32
			address := directHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != "" || r.Referer() != "" {
					t.Error("subscription request exposed controller credentials or redirect Referer")
				}
				if r.UserAgent() != "Jeemi subscription client" {
					t.Error("subscription headers were lost")
				}
				if r.URL.Path == "/sub" {
					http.Redirect(w, r, "http://subscription.invalid/current.yaml", http.StatusFound)
					return
				}
				w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100")
				_, _ = io.WriteString(w, "mode: rule\n")
			}))
			m := directDownloadManager(t, address)
			m.status.TUNEnabled, m.status.SystemProxy = tun, !tun
			client := m.NewDirectDownloadClient()
			if client == nil {
				t.Fatal("healthy direct route unavailable")
			}
			fetcher := subscription.NewHTTPFetcher()
			fetcher.Client.Transport = client.Transport
			fetched, err := fetcher.Fetch(context.Background(), "http://subscription.invalid/sub?token=fixture-secret")
			if err != nil || string(fetched.Contents) != "mode: rule\n" || fetched.RemoteProfile.DownloadBytes != 20 || requests.Load() != 2 {
				t.Fatalf("DIRECT fetch or redirected response parsing failed: %v, requests %d", err, requests.Load())
			}
		})
	}
}

func TestDirectDownloadRequiresHealthyCurrentGeneration(t *testing.T) {
	for _, state := range []State{StateStopped, StatePreparing, StateValidating, StateStarting, StateRunning, StateReloading, StateStopping, StateRecovering, StateFailed} {
		m := directDownloadManager(t, "127.0.0.1:12345")
		m.status.State = state
		if (m.NewDirectDownloadClient() != nil) != (state == StateRunning || state == StateReloading) {
			t.Fatalf("incorrect direct route availability for %s", state)
		}
	}
	for _, scenario := range []string{"unhealthy", "no generation", "no controller", "non-loopback", "missing password"} {
		t.Run(scenario, func(t *testing.T) {
			m := directDownloadManager(t, "127.0.0.1:12345")
			switch scenario {
			case "unhealthy":
				m.status.ControllerReady = false
			case "no generation":
				m.active = nil
			case "no controller":
				m.client = nil
			case "non-loopback":
				m.active.DNS.DirectAddress = "192.0.2.1:12345"
			case "missing password":
				m.active.DNS.Password = ""
			}
			if m.NewDirectDownloadClient() != nil {
				t.Fatal("invalid route was offered")
			}
		})
	}
}

func TestDirectDownloadRejectsOldSessionAndNeverFallsBackToLocalNetwork(t *testing.T) {
	var localRequests atomic.Int32
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localRequests.Add(1)
		_, _ = io.WriteString(w, "mode: rule\n")
	}))
	defer local.Close()
	for _, scenario := range []string{"connection failure", "stopped", "generation changed", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			// Port zero cannot be a working SOCKS endpoint.
			m := directDownloadManager(t, "127.0.0.1:0")
			fetcher := subscription.NewHTTPFetcher()
			fetcher.Client.Transport = m.NewDirectDownloadClient().Transport
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			switch scenario {
			case "stopped":
				m.status.State = StateStopped
			case "generation changed":
				m.active.ID = "new"
			case "cancelled":
				cancel()
			}
			_, err := fetcher.Fetch(ctx, local.URL+"/sub?token=fixture-secret")
			if err == nil || strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), m.active.DNS.Password) {
				t.Fatal("failed route succeeded or leaked credentials")
			}
		})
	}
	if localRequests.Load() != 0 {
		t.Fatal("DIRECT failure fell back to local sockets")
	}
}

// This SOCKS peer serves HTTP in memory and never dials the requested address.
// An unresolvable hostname proves that target DNS is delegated to mihomo.
func directHTTPFixture(t *testing.T, handler http.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			group.Add(1)
			go func() {
				defer group.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				read := func(size int) []byte {
					value := make([]byte, size)
					if _, err := io.ReadFull(conn, value); err != nil {
						t.Error("incomplete SOCKS handshake")
					}
					return value
				}
				head := read(2)
				_ = read(int(head[1]))
				_, _ = conn.Write([]byte{5, 2})
				auth := read(2)
				user := read(int(auth[1]))
				password := read(int(read(1)[0]))
				if auth[0] != 1 || string(user) != "jeemi-dns" || string(password) != strings.Repeat("d", 64) {
					t.Error("incorrect DIRECT channel authentication")
					return
				}
				_, _ = conn.Write([]byte{1, 0})
				connect := read(4)
				if connect[0] != 5 || connect[1] != 1 || connect[3] != 3 {
					t.Error("expected a hostname TCP CONNECT")
					return
				}
				host := read(int(read(1)[0]))
				port := binary.BigEndian.Uint16(read(2))
				if string(host) != "subscription.invalid" || port != 80 {
					t.Error("unexpected subscription destination")
					return
				}
				_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				request, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					t.Error("missing HTTP request inside DIRECT connection")
					return
				}
				defer request.Body.Close()
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				response := recorder.Result()
				defer response.Body.Close()
				_ = response.Write(conn)
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); group.Wait() })
	return listener.Addr().String()
}
