package macnetwork

import (
	"context"
	"jeemi/internal/platform/processmemory"
)

// Memory is passive: it neither registers a helper nor requests approval.
func Memory(ctx context.Context, webView bool) processmemory.HelperSnapshot {
	// The already authenticated XPC connection is sufficient on the hot path;
	// avoid rescanning bundle signatures on every runtime-status poll.
	reply, err := Call(ctx, Request{Operation: "memory", WebViewMemory: webView})
	if err == nil && reply.Memory != nil {
		return *reply.Memory
	}
	facts := nativeFacts()
	if !hasRegistration(facts) && (facts.Service == "not_registered" || facts.Service == "not_found") {
		return processmemory.AbsentHelper()
	}
	return processmemory.HelperSnapshot{}
}
