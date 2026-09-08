package daemon

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

type memoryJournal struct {
	state               JournalState
	failSave, failClear bool
	saves               int
}

func clone[T any](value T) T {
	data, _ := json.Marshal(value)
	var copy T
	_ = json.Unmarshal(data, &copy)
	return copy
}
func (m *memoryJournal) Load() (JournalState, error) { return clone(m.state), nil }
func (m *memoryJournal) Save(state JournalState) error {
	if m.failSave {
		return errors.New("save failed")
	}
	m.state = clone(state)
	m.saves++
	return nil
}
func (m *memoryJournal) Clear() error {
	if m.failClear {
		return errors.New("clear failed")
	}
	m.state = JournalState{}
	return nil
}

type memoryNetwork struct {
	values                 Services
	writes                 int
	failWrite, ignoreWrite bool
}

func (m *memoryNetwork) Read() (Services, error) { return clone(m.values), nil }
func (m *memoryNetwork) Write(changes Services) error {
	m.writes++
	if m.failWrite {
		return errors.New("write failed")
	}
	if m.ignoreWrite {
		return nil
	}
	for id, change := range changes {
		service, ok := m.values[id]
		if !ok {
			continue
		}
		if change.Proxy != nil {
			service.Proxy = clone(change.Proxy)
		}
		if change.DNS != nil {
			service.DNS = clone(change.DNS)
		}
		m.values[id] = service
	}
	return nil
}
func networkFixture() (*memoryJournal, *memoryNetwork) {
	return &memoryJournal{}, &memoryNetwork{values: Services{"wifi": {Active: true, Proxy: Values{"HTTPEnable": 1, "HTTPProxy": "old.example", "HTTPPort": 8080, "ProxyAutoConfigEnable": 1, "SOCKSProxy": nil}, DNS: Values{"ServerAddresses": []string{"1.1.1.1"}}}}}
}

func TestProxyDNSRestorationAndNetworkChanges(t *testing.T) {
	store, network := networkFixture()
	original := clone(network.values)
	journal := NewJournal(store, network)
	if err := journal.ApplyProxy("http=127.0.0.1:7890;https=127.0.0.1:7890"); err != nil {
		t.Fatal(err)
	}
	if err := journal.ApplyDNS("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	network.values["ethernet"] = NetworkService{Active: true, Proxy: Values{"HTTPEnable": 0}, DNS: Values{"ServerAddresses": nil}}
	original["ethernet"] = clone(network.values["ethernet"])
	if err := journal.Refresh(); err != nil {
		t.Fatal(err)
	}
	if err := journal.Restore(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(network.values, original) {
		t.Fatalf("original settings were not restored: %#v", network.values)
	}
	writes := network.writes
	if err := journal.Restore(); err != nil || network.writes != writes {
		t.Fatal("restore is not idempotent")
	}
}
func TestJournalPersistsBeforeWritingAndRetainsFailedRecovery(t *testing.T) {
	store, network := networkFixture()
	journal := NewJournal(store, network)
	store.failSave = true
	if journal.ApplyProxy("socks=127.0.0.1:7891") == nil || network.writes != 0 {
		t.Fatal("network changed without recovery journal")
	}
	store.failSave = false
	network.ignoreWrite = true
	if journal.ApplyDNS("127.0.0.1") == nil || len(store.state.Original) == 0 {
		t.Fatal("readback not checked")
	}
	network.ignoreWrite = false
	if err := journal.ApplyDNS("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	network.failWrite = true
	if journal.Restore() == nil || len(store.state.Original) == 0 {
		t.Fatal("failed restoration erased journal")
	}
	network.failWrite = false
	if err := journal.Restore(); err != nil || len(store.state.Original) != 0 {
		t.Fatal("recovery retry failed")
	}
}
func TestProxyEndpointIsLoopbackOnly(t *testing.T) {
	for _, endpoint := range []string{"http=evil:1;https=evil:1", "http=127.0.0.1:80", "socks=127.0.0.1:0", "socks=127.0.0.1:1;socks=127.0.0.1:2", ""} {
		if _, err := ProxyValues(endpoint); err == nil {
			t.Fatal(endpoint)
		}
	}
	values, err := ProxyValues("socks=127.0.0.1:7891")
	if err != nil || values["HTTPEnable"] != 0 || values["SOCKSEnable"] != 1 {
		t.Fatal(values, err)
	}
}
