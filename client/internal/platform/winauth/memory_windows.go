//go:build windows

package winauth

import (
	"context"

	"golang.org/x/sys/windows"
	"jeemi/internal/platform/processmemory"
)

// Memory never installs, starts or elevates a service. A failed/old service is
// unknown; only SCM's explicit absence can contribute a measured zero.
func Memory(ctx context.Context, webView bool) processmemory.HelperSnapshot {
	sid, err := ownerSID()
	if err != nil {
		return processmemory.HelperSnapshot{}
	}
	service, err := queryService(sid)
	if err == windows.ERROR_SERVICE_DOES_NOT_EXIST {
		return processmemory.AbsentHelper()
	}
	if err != nil {
		return processmemory.HelperSnapshot{}
	}
	service.Close()
	reply, err := Call(ctx, Request{Operation: "memory", WebViewMemory: webView})
	if err != nil || reply.Memory == nil {
		return processmemory.HelperSnapshot{}
	}
	return *reply.Memory
}
