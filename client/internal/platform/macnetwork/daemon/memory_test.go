package daemon

import (
	"encoding/json"
	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/platform/processmemory"
	"testing"
)

func TestMemoryUsesOwnedProcessAndAuthenticatedWebClient(t *testing.T) {
	server, cores, _, _ := fixtureServer()
	if request(t, server, 1, "start").Code != "ok" {
		t.Fatal("start failed")
	}
	helperBytes, coreBytes, webBytes := uint64(11), uint64(22), uint64(33)
	calls := 0
	server.memory = func(pid int) processmemory.HelperSnapshot {
		calls++
		if pid != cores.process.PID() {
			t.Fatal("memory uses an unowned core")
		}
		return processmemory.HelperSnapshot{HelperBytes: &helperBytes, MihomoBytes: &coreBytes, MihomoPID: pid}
	}
	server.webMemory = func(pid, core int) *uint64 {
		if pid != 77 || core != cores.process.PID() {
			t.Fatal("web memory lost authenticated identity")
		}
		return &webBytes
	}
	data, _ := json.Marshal(macnetwork.Request{Protocol: macnetwork.ProtocolVersion, Operation: "memory", WebViewMemory: true})
	var reply macnetwork.Response
	_ = json.Unmarshal(server.Handle(2, 501, 88, data), &reply)
	if reply.Code != "busy" || calls != 0 {
		t.Fatal("another client queried the owned core")
	}
	server.failedRecovery = true // Memory is independent of network recovery.
	_ = json.Unmarshal(server.Handle(1, 501, 77, data), &reply)
	if reply.Code != "ok" || reply.Memory == nil || *reply.Memory.HelperBytes != 11 || *reply.Memory.MihomoBytes != 22 || *reply.Memory.WebViewBytes != 33 {
		t.Fatalf("bad memory reply %+v", reply)
	}
}

func TestMemoryRejectsQueryArgumentsAndDiscardsExitedProcess(t *testing.T) {
	server, cores, _, _ := fixtureServer()
	for _, input := range []string{
		`{"protocol":3,"operation":"memory","pid":1}`,
		`{"protocol":3,"operation":"memory","target":{}}`,
		`{"protocol":3,"operation":"memory","configuration":"config.yaml"}`,
		`{"protocol":3,"operation":"ping","webViewMemory":true}`,
	} {
		var reply macnetwork.Response
		_ = json.Unmarshal(server.Handle(1, 501, 77, []byte(input)), &reply)
		if reply.Code != "invalid_request" {
			t.Fatal("unsafe memory query accepted")
		}
	}
	request(t, server, 1, "start")
	server.memory = func(pid int) processmemory.HelperSnapshot {
		cores.process.running = false
		bytes := uint64(999)
		return processmemory.HelperSnapshot{MihomoPID: pid, MihomoBytes: &bytes}
	}
	data, _ := json.Marshal(macnetwork.Request{Protocol: macnetwork.ProtocolVersion, Operation: "memory"})
	var reply macnetwork.Response
	_ = json.Unmarshal(server.Handle(1, 501, 77, data), &reply)
	if reply.Code != "ok" || reply.Memory == nil || reply.Memory.MihomoBytes != nil {
		t.Fatal("exited core retained a measurement")
	}
}
