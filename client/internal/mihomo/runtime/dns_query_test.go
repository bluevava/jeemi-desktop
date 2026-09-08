package runtime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/dnsquery"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSProxyLeafFollowsCurrentSelectionsAndRejectsNonProxy(t *testing.T) {
	states := map[string]proxySelectionState{
		"Proxy": {Type: "Selector", Now: "Auto", All: []string{"Auto", "DIRECT"}},
		"Auto":  {Type: "URLTest", Now: "Node", All: []string{"Node"}},
		"Node":  {Type: "Shadowsocks"}, "DIRECT": {Type: "Direct"},
	}
	if node, code := dnsProxyLeaf("Proxy", states); node != "Node" || code != "" {
		t.Fatalf("node = %s, %s", node, code)
	}
	states["Auto"] = proxySelectionState{Type: "Fallback", Now: "DIRECT", All: []string{"DIRECT"}}
	if _, code := dnsProxyLeaf("Proxy", states); code != "proxy_invalid" {
		t.Fatal("accepted DIRECT as a proxy")
	}
	states["Auto"] = proxySelectionState{Type: "LoadBalance", All: []string{"Node"}}
	if _, code := dnsProxyLeaf("Proxy", states); code != "ambiguous_proxy" {
		t.Fatal("invented a current load-balancing node")
	}
	states["Auto"] = proxySelectionState{Type: "Selector", Now: "Proxy", All: []string{"Proxy"}}
	if _, code := dnsProxyLeaf("Proxy", states); code != "proxy_invalid" {
		t.Fatal("accepted selection cycle")
	}
}

func TestDNSProviderLeavesRejectAmbiguousNamesAndIgnoreImplicitProvider(t *testing.T) {
	client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		body := `{"providers":{"implicit":{"proxies":[{"name":"Unique","type":"Shadowsocks"}]},"first":{"proxies":[{"name":"Unique","type":"Shadowsocks"},{"name":"Duplicate","type":"Vless"}]},"second":{"proxies":[{"name":"Duplicate","type":"Trojan"},{"name":"Root","type":"Vless"}]}}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	states := map[string]proxySelectionState{"Root": {Type: "Shadowsocks"}}
	controller := newControllerClient(client, ControllerSession{BaseURL: "http://127.0.0.1:9999"})
	if err := controller.addDNSProviderLeaves(context.Background(), states, []string{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	if name, code := dnsProxyLeaf("Unique", states); name != "Unique" || code != "" {
		t.Fatal("counted an implicit provider twice")
	}
	for _, name := range []string{"Duplicate", "Root"} {
		if _, code := dnsProxyLeaf(name, states); code != "ambiguous_proxy" {
			t.Fatalf("ambiguous %s accepted", name)
		}
	}
}

func TestDNSUsesDedicatedRoutesAndPinsOnlyPrivateSelector(t *testing.T) {
	password := strings.Repeat("a", 64)
	proxyAddress, proxyCount := dnsSOCKSFixture(t, password)
	directAddress, directCount := dnsSOCKSFixture(t, password)
	var selected atomic.Int32
	client := &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer "+password {
			t.Error("missing API authentication")
		}
		body := ""
		switch {
		case request.Method == "GET" && request.URL.Path == "/proxies":
			body = `{"proxies":{"Proxy":{"type":"Selector","now":"Node","all":["Node"]},"Node":{"type":"Shadowsocks"},"__jeemi_dns_egress":{"type":"Selector","now":"REJECT","all":["REJECT","Node"]}}}`
		case request.Method == "PUT" && request.URL.Path == "/proxies/"+dnstool.Group:
			var value map[string]string
			_ = json.NewDecoder(request.Body).Decode(&value)
			if value["name"] != "Node" {
				t.Error("wrong pinned node")
			}
			selected.Add(1)
		default:
			t.Errorf("changed user routing or made unexpected API request: %s %s", request.Method, request.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	manager, err := NewManager(Options{DataDirectory: t.TempDir(), HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	session := ControllerSession{ID: "session", BaseURL: "http://127.0.0.1:9999", Secret: password}
	manager.status = Status{State: StateRunning, ControllerReady: true, TUNEnabled: true}
	manager.active = &Generation{ID: "generation", Session: session, DNS: dnstool.Session{ProxyAddress: proxyAddress, DirectAddress: directAddress, Username: "jeemi-dns", Password: password, MatchTarget: "Proxy"}}
	manager.client = newControllerClient(client, session)
	response, err := manager.QueryDNS(context.Background(), dnsquery.Request{ID: "query", Domain: "example.com", ProxyDNS: "192.0.2.1", DirectDNS: "192.0.2.2", CustomDNS: "192.0.2.3", CustomProxy: true})
	if err != nil {
		t.Fatal(err)
	}
	if response.Proxy.Status != "success" || response.Direct.Status != "success" || response.Custom.Status != "success" {
		t.Fatalf("response = %+v", response)
	}
	if response.Proxy.Node != "Node" || response.Proxy.Selector != "Proxy" || response.Custom.Route != "proxy" || response.Direct.Route != "direct" {
		t.Fatalf("route metadata = %+v", response)
	}
	if proxyCount.Load() != 4 || directCount.Load() != 2 || selected.Load() != 1 {
		t.Fatalf("wrong egress counts: %d %d %d", proxyCount.Load(), directCount.Load(), selected.Load())
	}
}

func TestDNSOfflineProxyNeverFallsBackAndCancelIsScoped(t *testing.T) {
	manager, err := NewManager(Options{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	request := dnsquery.Request{ID: "offline", Domain: "example.com", ProxyDNS: "192.0.2.1", DirectDNS: "invalid", CustomDNS: "192.0.2.2", CustomProxy: true}
	manager.CancelDNSQuery(request.ID)
	if _, err := manager.QueryDNS(context.Background(), request); err == nil || err.Error() != "cancelled" {
		t.Fatal("cancellation before Wails dispatch was lost")
	}
	response, err := manager.QueryDNS(context.Background(), request)
	if err != nil || response.Proxy.Error != "proxy_invalid" || response.Custom.Error != "proxy_invalid" || response.Direct.Error != "invalid_server" {
		t.Fatalf("offline = %+v %v", response, err)
	}
	entered := make(chan struct{})
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	manager.status = Status{State: StateRunning, ControllerReady: true}
	manager.active = &Generation{ID: "generation", DNS: dnstool.Session{MatchTarget: "Proxy"}}
	manager.client = newControllerClient(client, ControllerSession{BaseURL: "http://127.0.0.1:9999"})
	request.ID = "pending"
	done := make(chan error, 1)
	go func() { _, err := manager.QueryDNS(context.Background(), request); done <- err }()
	<-entered
	if _, err := manager.QueryDNS(context.Background(), request); err == nil || err.Error() != "busy" {
		t.Fatal("allowed overlapping queries")
	}
	manager.CancelDNSQuery("old-request")
	select {
	case <-done:
		t.Fatal("old cancellation affected active request")
	default:
	}
	manager.CancelDNSQuery("pending")
	select {
	case err := <-done:
		if err == nil || err.Error() != "cancelled" {
			t.Fatalf("cancel = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("query did not cancel")
	}
}

// This test server terminates SOCKS and answers synthetic DNS packets entirely
// on loopback. It never dials the requested destination or changes OS networking.
func dnsSOCKSFixture(t *testing.T, password string) (string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	count := &atomic.Int32{}
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
				var head [4]byte
				if _, err := io.ReadFull(conn, head[:2]); err != nil {
					return
				}
				methods := make([]byte, int(head[1]))
				_, _ = io.ReadFull(conn, methods)
				_, _ = conn.Write([]byte{5, 2})
				_, _ = io.ReadFull(conn, head[:2])
				user := make([]byte, int(head[1]))
				_, _ = io.ReadFull(conn, user)
				_, _ = io.ReadFull(conn, head[:1])
				pass := make([]byte, int(head[0]))
				_, _ = io.ReadFull(conn, pass)
				if string(user) != "jeemi-dns" || string(pass) != password {
					t.Error("incorrect SOCKS credentials")
					return
				}
				_, _ = conn.Write([]byte{1, 0})
				_, _ = io.ReadFull(conn, head[:])
				if head[0] != 5 || head[1] != 1 || head[3] != 1 {
					t.Error("expected IPv4 TCP CONNECT")
					return
				}
				destination := make([]byte, 6)
				_, _ = io.ReadFull(conn, destination)
				if port := binary.BigEndian.Uint16(destination[4:]); port != 53 {
					t.Error("unexpected DNS port " + strconv.Itoa(int(port)))
					return
				}
				_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				_, _ = io.ReadFull(conn, head[:2])
				packet := make([]byte, binary.BigEndian.Uint16(head[:2]))
				_, _ = io.ReadFull(conn, packet)
				var query dnsmessage.Message
				if query.Unpack(packet) != nil || len(query.Questions) != 1 {
					t.Error("invalid query")
					return
				}
				count.Add(1)
				query.Response, query.RecursionAvailable = true, true
				packet, _ = query.Pack()
				binary.BigEndian.PutUint16(head[:2], uint16(len(packet)))
				_, _ = conn.Write(append(head[:2], packet...))
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); group.Wait() })
	return listener.Addr().String(), count
}
