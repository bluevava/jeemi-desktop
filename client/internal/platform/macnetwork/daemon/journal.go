package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	"jeemi/internal/platform/macnetwork"
)

type Values map[string]any
type NetworkService struct {
	Active bool   `json:"active"`
	Proxy  Values `json:"proxy,omitempty"`
	DNS    Values `json:"dns,omitempty"`
}
type Services map[string]NetworkService
type JournalState struct {
	Version  int      `json:"version"`
	Original Services `json:"original"`
}
type JournalStore interface {
	Load() (JournalState, error)
	Save(JournalState) error
	Clear() error
}
type NetworkBackend interface {
	Read() (Services, error)
	Write(Services) error
}

// Journal is owned by the daemon, never supplied by the GUI. Original values
// reach durable storage before every first mutation, including new services.
type Journal struct {
	store   JournalStore
	backend NetworkBackend
	proxy   Values
	dns     Values
}

func NewJournal(store JournalStore, backend NetworkBackend) *Journal {
	return &Journal{store: store, backend: backend}
}

func ProxyValues(endpoint string) (Values, error) {
	values := Values{"HTTPEnable": 0, "HTTPSEnable": 0, "SOCKSEnable": 0, "FTPEnable": 0, "RTSPEnable": 0, "GopherEnable": 0, "ProxyAutoConfigEnable": 0, "ProxyAutoDiscoveryEnable": 0, "ExcludeSimpleHostnames": 1,
		"HTTPProxy": nil, "HTTPPort": nil, "HTTPSProxy": nil, "HTTPSPort": nil, "SOCKSProxy": nil, "SOCKSPort": nil,
		"ExceptionsList": []string{"localhost", "127.0.0.1", "::1", "*.local", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "fc00::/7", "fe80::/10"}}
	seen := map[string]bool{}
	for _, part := range strings.Split(endpoint, ";") {
		protocol, address, ok := strings.Cut(part, "=")
		prefix := map[string]string{"http": "HTTP", "https": "HTTPS", "socks": "SOCKS"}[protocol]
		host, port, err := net.SplitHostPort(address)
		number, parseErr := strconv.Atoi(port)
		if !ok || prefix == "" || seen[protocol] || err != nil || parseErr != nil || host != "127.0.0.1" || number < 1 || number > 65535 {
			return nil, macnetwork.Failure("invalid_request")
		}
		seen[protocol] = true
		values[prefix+"Enable"] = 1
		values[prefix+"Proxy"] = host
		values[prefix+"Port"] = number
	}
	if seen["http"] != seen["https"] {
		return nil, macnetwork.Failure("invalid_request")
	}
	return values, nil
}

func (j *Journal) ApplyProxy(endpoint string) error {
	values, err := ProxyValues(endpoint)
	if err != nil {
		return err
	}
	previous := j.proxy
	j.proxy = values
	if err = j.Refresh(); err != nil {
		j.proxy = previous
		return err
	}
	return nil
}
func (j *Journal) ApplyDNS(address string) error {
	if address != "127.0.0.1" && address != "::1" {
		return macnetwork.Failure("dns_invalid")
	}
	previous := j.dns
	j.dns = Values{"ServerAddresses": []string{address}}
	if err := j.Refresh(); err != nil {
		j.dns = previous
		return err
	}
	return nil
}

func (j *Journal) Refresh() error {
	if j.proxy == nil && j.dns == nil {
		return nil
	}
	current, err := j.backend.Read()
	if err != nil {
		return err
	}
	state, err := j.store.Load()
	if err != nil {
		return err
	}
	if state.Original == nil {
		state = JournalState{Version: 1, Original: Services{}}
	}
	changes := Services{}
	active := 0
	changedJournal := false
	for id, service := range current {
		if !service.Active {
			continue
		}
		active++
		original := state.Original[id]
		change := NetworkService{}
		if j.proxy != nil {
			if original.Proxy == nil {
				original.Proxy = service.Proxy
				changedJournal = true
			}
			change.Proxy = j.proxy
		}
		if j.dns != nil {
			if original.DNS == nil {
				original.DNS = service.DNS
				changedJournal = true
			}
			change.DNS = j.dns
		}
		state.Original[id] = original
		if !wireEqual(change.Proxy, service.Proxy) || !wireEqual(change.DNS, service.DNS) {
			changes[id] = change
		}
	}
	if active == 0 {
		return macnetwork.Failure("network_failed")
	}
	if changedJournal {
		if err = j.store.Save(state); err != nil {
			return err
		}
	}
	if len(changes) == 0 {
		return nil
	}
	if err = j.backend.Write(changes); err != nil {
		return err
	}
	return j.verify(changes)
}

func (j *Journal) Restore() error {
	j.proxy, j.dns = nil, nil
	state, err := j.store.Load()
	if err != nil {
		return err
	}
	if len(state.Original) == 0 {
		return nil
	}
	if err = j.backend.Write(state.Original); err != nil {
		return err
	}
	if err = j.verify(state.Original); err != nil {
		return err
	}
	return j.store.Clear()
}

func (j *Journal) verify(expected Services) error {
	actual, err := j.backend.Read()
	if err != nil {
		return err
	}
	for id, want := range expected {
		got, exists := actual[id]
		if !exists {
			continue
		}
		for name, pair := range map[string][2]Values{"proxy": {want.Proxy, got.Proxy}, "dns": {want.DNS, got.DNS}} {
			for key, value := range pair[0] {
				// Compare wire values so int/float64 and []string/[]any agree.
				a, _ := json.Marshal(value)
				b, _ := json.Marshal(pair[1][key])
				if !reflect.DeepEqual(a, b) {
					return fmt.Errorf("network %s readback failed", name)
				}
			}
		}
	}
	return nil
}

func wireEqual(want, got Values) bool {
	if want == nil {
		return true
	}
	for key, value := range want {
		a, _ := json.Marshal(value)
		b, _ := json.Marshal(got[key])
		if !reflect.DeepEqual(a, b) {
			return false
		}
	}
	return true
}
