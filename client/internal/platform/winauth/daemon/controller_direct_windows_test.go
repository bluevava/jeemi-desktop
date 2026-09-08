//go:build windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
	"jeemi/internal/config/runtimecontrol"
	"jeemi/internal/platform/coresnapshot"
	"jeemi/internal/platform/winauth"
)

func TestServicePreservesDirectControllerThroughStartAndReload(t *testing.T) {
	target, _, cores, workspace, configuration := selectedCoreFixture(t, true)
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	port := uint16(reservation.Addr().(*net.TCPAddr).Port)
	reservation.Close()
	secret := strings.Repeat("s", 64)
	origins := runtimecontrol.DefaultAllowedOrigins()
	source := t.TempDir()
	stage := func(relative, mode string, tun, initial bool) {
		t.Helper()
		input := controllerFixtureConfiguration{Address: address, Secret: secret, Mode: mode}
		input.CORS.Origins = origins
		input.TUN.Enable = tun
		data, err := yaml.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(source, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		var archive bytes.Buffer
		if err := coresnapshot.WriteSnapshot(source, relative, initial, &archive); err != nil {
			t.Fatal(err)
		}
		if err := cores.snapshot(workspace, Request{Configuration: relative, SourceRoot: source, SnapshotSize: int64(archive.Len())}, &archive, initial); err != nil {
			t.Fatal(err)
		}
	}
	stage(configuration, "rule", false, true)
	if err := os.WriteFile(filepath.Join(workspace, "test-controller.enabled"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := winauth.OpenInstalledCore(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := cores.start(file, workspace, configuration); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cores.stop(ctx); err != nil {
			t.Error(err)
		}
	})
	assertTestCoreImage(t, workspace, target.ExecutablePath)
	if !ownsTCP(uint32(cores.command.Process.Pid), port, true) || ownsTCP(uint32(os.Getpid()), port, true) {
		t.Fatal("GUI controller address is not owned directly by the core")
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 3 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	request := func(method, path, origin, body string, preflight bool) []byte {
		t.Helper()
		r, err := http.NewRequest(method, "http://"+address+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Origin", origin)
		if preflight {
			r.Header.Set("Access-Control-Request-Method", http.MethodDelete)
			r.Header.Set("Access-Control-Request-Headers", "authorization")
		} else {
			r.Header.Set("Authorization", "Bearer "+secret)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode >= 300 || response.Header.Get("Access-Control-Allow-Origin") != origin {
			t.Fatalf("direct %s %s failed: HTTP %d", method, path, response.StatusCode)
		}
		if preflight && (!strings.Contains(response.Header.Get("Access-Control-Allow-Methods"), http.MethodDelete) || !strings.Contains(response.Header.Get("Access-Control-Allow-Headers"), "Authorization")) {
			t.Fatal("core preflight response did not reach the GUI")
		}
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	checkConfiguration := func(mode string, tun bool) {
		t.Helper()
		var current controllerFixtureConfiguration
		if json.Unmarshal(request(http.MethodGet, "/configs", origins[0], "", false), &current) != nil || current.Address != address || current.Secret != secret || !slices.Equal(current.CORS.Origins, origins) || current.Mode != mode || current.TUN.Enable != tun {
			t.Fatal("controller session or reloaded configuration changed")
		}
	}
	checkConfiguration("rule", false)
	for _, origin := range origins {
		request(http.MethodPut, "/proxies/"+url.PathEscape("🌐 Selector"), origin, `{"name":"New Node"}`, false)
		var snapshot struct{ Connections []struct{ ID string } }
		if json.Unmarshal(request(http.MethodGet, "/connections", origin, "", false), &snapshot) != nil || len(snapshot.Connections) != 1 {
			t.Fatal("direct connection snapshot failed")
		}
		path := "/connections/" + url.PathEscape(snapshot.Connections[0].ID)
		request(http.MethodOptions, path, origin, "", true)
		request(http.MethodDelete, path, origin, "", false)
	}
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	connection, _, err := dialer.Dial("ws://"+address+"/traffic?token="+url.QueryEscape(secret), http.Header{"Origin": []string{origins[0]}})
	if err != nil {
		t.Fatal("direct WebSocket connection failed")
	}
	var traffic struct{ Up, Down int }
	err = connection.ReadJSON(&traffic)
	connection.Close()
	if err != nil || traffic.Up != 123 || traffic.Down != 456 {
		t.Fatal("direct WebSocket did not use the generation credential")
	}
	next := "generations/two/config.yaml"
	stage(next, "global", true, false)
	payload, err := json.Marshal(map[string]string{"path": filepath.Join(workspace, filepath.FromSlash(next))})
	if err != nil {
		t.Fatal(err)
	}
	request(http.MethodPut, "/configs?force=true", origins[0], string(payload), false)
	checkConfiguration("global", true)
	cores.disableTUN(context.Background())
	checkConfiguration("global", false)
}
