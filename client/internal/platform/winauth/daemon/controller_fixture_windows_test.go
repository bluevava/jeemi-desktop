//go:build windows

package daemon

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

type controllerFixtureConfiguration struct {
	Address string `yaml:"external-controller" json:"external-controller"`
	Secret  string `yaml:"secret" json:"secret"`
	Mode    string `yaml:"mode" json:"mode"`
	CORS    struct {
		Origins []string `yaml:"allow-origins" json:"allow-origins"`
	} `yaml:"external-controller-cors" json:"external-controller-cors"`
	TUN struct {
		Enable bool `yaml:"enable" json:"enable"`
	} `yaml:"tun" json:"tun"`
}

func readControllerFixtureConfiguration(path string) (controllerFixtureConfiguration, error) {
	var configuration controllerFixtureConfiguration
	data, err := os.ReadFile(path)
	if err == nil {
		err = yaml.Unmarshal(data, &configuration)
	}
	return configuration, err
}

func startControllerFixture(path string) (*http.Server, error) {
	configuration, err := readControllerFixtureConfiguration(path)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", configuration.Address)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !slices.Contains(configuration.CORS.Origins, origin) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		secret := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if websocket.IsWebSocketUpgrade(r) {
			secret = r.URL.Query().Get("token")
		}
		if secret != configuration.Secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case websocket.IsWebSocketUpgrade(r) && r.URL.Path == "/traffic":
			upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return origin != "" }}
			connection, err := upgrader.Upgrade(w, r, nil)
			if err == nil {
				defer connection.Close()
				_ = connection.WriteJSON(map[string]int{"up": 123, "down": 456})
			}
		case r.Method == http.MethodGet && r.URL.Path == "/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "test-core"})
		case r.Method == http.MethodGet && r.URL.Path == "/configs":
			_ = json.NewEncoder(w).Encode(configuration)
		case r.Method == http.MethodPut && r.URL.Path == "/configs":
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 1 || r.URL.Query().Get("force") != "true" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			next, err := readControllerFixtureConfiguration(body["path"])
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			configuration = next
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/configs":
			var body struct {
				TUN struct{ Enable bool } `json:"tun"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			configuration.TUN.Enable = body.TUN.Enable
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/proxies/🌐 Selector":
			var body struct{ Name string }
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.Name != "New Node" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/connections":
			_, _ = w.Write([]byte(`{"connections":[{"id":"test-id","chains":["Old Node","🌐 Selector"]}]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/connections/test-id":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	return server, nil
}
