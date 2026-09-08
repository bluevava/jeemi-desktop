package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"jeemi/internal/platform/macnetwork"
	"testing"
	"time"
)

type fakeCore struct {
	process *fakeProcess
	prepare func(context.Context) error
	starts  int
}
type fakeProcess struct {
	running bool
	onStop  func()
}

func (p *fakeProcess) PID() int      { return 1001 }
func (p *fakeProcess) Running() bool { return p.running }
func (p *fakeProcess) Stop() error {
	if p.onStop != nil {
		p.onStop()
	}
	p.running = false
	return nil
}
func (c *fakeCore) Prepare(ctx context.Context, _ macnetwork.Target) error {
	if c.prepare != nil {
		return c.prepare(ctx)
	}
	return nil
}
func (c *fakeCore) Check(t macnetwork.Target) error { return t.Validate() }
func (c *fakeCore) Start(_ uint32, t macnetwork.Target, _ string) (Process, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	c.starts++
	c.process = &fakeProcess{running: true}
	return c.process, nil
}
func (c *fakeCore) Device() (string, error) { return "utun7", nil }

var testTarget = macnetwork.Target{Version: "v1.19.0", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1024}

func request(t *testing.T, s *Server, client uint64, op string) macnetwork.Response {
	t.Helper()
	data, _ := json.Marshal(macnetwork.Request{Protocol: macnetwork.ProtocolVersion, Operation: op, Target: &testTarget, Configuration: "generations/one/bootstrap.yaml", Endpoint: "socks=127.0.0.1:7891"})
	var reply macnetwork.Response
	if json.Unmarshal(s.Handle(client, 501, 77, data), &reply) != nil {
		t.Fatal("bad response")
	}
	return reply
}
func fixtureServer() (*Server, *fakeCore, *memoryJournal, *memoryNetwork) {
	store, network := networkFixture()
	cores := &fakeCore{}
	server := NewServer(cores, NewJournal(store, network), func(string) (DNSGateway, string, error) { return nil, "", errors.New("unexpected real DNS") })
	server.Connected(1)
	server.Connected(2)
	return server, cores, store, network
}
func TestOwnershipDisconnectAndRecoveryOrder(t *testing.T) {
	server, cores, store, network := fixtureServer()
	original := clone(network.values)
	if request(t, server, 1, "start").Code != "ok" || request(t, server, 1, "proxy").Code != "ok" {
		t.Fatal("start failed")
	}
	if request(t, server, 2, "stop").Code != "busy" {
		t.Fatal("another client stopped owner")
	}
	cores.process.onStop = func() {
		if len(store.state.Original) != 0 {
			t.Error("core stopped before network recovery")
		}
	}
	server.Disconnected(1)
	if cores.process.running || len(store.state.Original) != 0 {
		t.Fatal("disconnected client left proxy running")
	}
	if network.values["wifi"].Proxy["HTTPProxy"] != original["wifi"].Proxy["HTTPProxy"] {
		t.Fatal("proxy not restored")
	}
	if request(t, server, 1, "start").Code == "ok" || cores.starts != 1 {
		t.Fatal("queued request started after disconnect")
	}
}
func TestCancelPreparationPreservesHealthyCore(t *testing.T) {
	server, cores, _, _ := fixtureServer()
	request(t, server, 1, "start")
	entered := make(chan struct{})
	cores.prepare = func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }
	done := make(chan macnetwork.Response, 1)
	go func() { done <- request(t, server, 1, "prepare") }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("prepare not entered")
	}
	request(t, server, 1, "cancel_prepare")
	select {
	case result := <-done:
		if result.Code != "cancelled" {
			t.Fatal(result)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not finish")
	}
	if !cores.process.running {
		t.Fatal("cancel stopped healthy core")
	}
	server.Close()
}
func TestCoreCrashRestoresAndNeverAutostarts(t *testing.T) {
	server, cores, store, network := fixtureServer()
	request(t, server, 1, "start")
	request(t, server, 1, "proxy")
	cores.process.running = false
	network.failWrite = true
	server.Tick()
	if len(store.state.Original) == 0 || request(t, server, 1, "start").Code != "recovery_failed" {
		t.Fatal("recovery gate lost")
	}
	network.failWrite = false
	server.Tick()
	if len(store.state.Original) != 0 || cores.starts != 1 {
		t.Fatal("recovery started a core")
	}
	server.Close()
}
func TestProtocolRejectsUnknownCommandsAndFields(t *testing.T) {
	server, _, _, _ := fixtureServer()
	defer server.Close()
	for _, input := range []string{`{"protocol":2,"operation":"exec","command":"id"}`, `{"protocol":2,"operation":"stop","pid":99}`, `{"protocol":1,"operation":"start"}`, `{"protocol":2,"operation":"ping"} {}`} {
		var response macnetwork.Response
		_ = json.Unmarshal(server.Handle(1, 501, 77, []byte(input)), &response)
		if response.Code != "invalid_request" {
			t.Fatal(input, response)
		}
	}
}

func (c *fakeCore) VerifyProxy(string) error { return nil }
func (c *fakeCore) VerifyDNS(string) error   { return nil }
