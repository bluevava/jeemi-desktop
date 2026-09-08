package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestControllerClientUsesBearerAndOfficialMutationShapes(t *testing.T) {
	secret := strings.Repeat("s", 64)
	var calls []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+secret {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		entry := map[string]any{"method": request.Method, "path": request.URL.Path, "query": request.URL.Query()}
		if request.Body != nil {
			var payload any
			_ = json.NewDecoder(request.Body).Decode(&payload)
			entry["payload"] = payload
		}
		calls = append(calls, entry)
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/version" {
			_, _ = response.Write([]byte(`{"version":"v1.19.30","meta":true}`))
			return
		}
		if request.URL.Path == "/configs" && request.Method == http.MethodGet {
			_, _ = response.Write([]byte(`{"mode":"rule"}`))
			return
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := newControllerClient(server.Client(), ControllerSession{BaseURL: server.URL, Secret: secret})
	if err := client.WaitHealthy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.Reload(context.Background(), `C:\runtime\config.yaml`); err != nil {
		t.Fatal(err)
	}
	if err := client.SetTUN(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := client.SetMode(context.Background(), "global"); err != nil {
		t.Fatal(err)
	}
	if err := client.SelectProxy(context.Background(), "Main Group", "Node A"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 6 {
		t.Fatalf("calls = %#v", calls)
	}
	if calls[2]["method"] != http.MethodPut || calls[2]["path"] != "/configs" {
		t.Fatalf("reload call = %#v", calls[2])
	}
	reloadQuery, ok := calls[2]["query"].(url.Values)
	if !ok || reloadQuery.Get("force") != "true" || reloadQuery.Get("path") != "" {
		t.Fatalf("reload query = %#v", reloadQuery)
	}
	reloadPayload, ok := calls[2]["payload"].(map[string]any)
	if !ok || len(reloadPayload) != 1 || reloadPayload["path"] != `C:\runtime\config.yaml` {
		t.Fatalf("reload payload = %#v", calls[2]["payload"])
	}
	if calls[4]["method"] != http.MethodPatch {
		t.Fatalf("mode call = %#v", calls[4])
	}
	if calls[5]["path"] != "/proxies/Main Group" {
		t.Fatalf("proxy call = %#v", calls[5])
	}
}
