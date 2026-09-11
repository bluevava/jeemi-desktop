package application

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"jeemi/internal/iconcache"
)

type iconTransport func(*http.Request) (*http.Response, error)

func (f iconTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSelectorIconCancellationAndShutdown(t *testing.T) {
	started := make(chan struct{}, 1)
	s := &Service{ctx: context.Background(), iconCancels: make(map[string]context.CancelFunc)}
	s.selectorIcons = iconcache.New(t.TempDir(), func() *http.Client {
		return &http.Client{Transport: iconTransport(func(r *http.Request) (*http.Response, error) {
			started <- struct{}{}
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}
	})
	for _, id := range []string{"request-1", "request-2"} {
		done := make(chan error, 1)
		go func() { _, err := s.SelectorIcon(id, "https://assets.example/icon.png"); done <- err }()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("download did not start")
		}
		if id == "request-1" {
			s.CancelSelectorIcon(id)
		} else {
			s.cancelSelectorIcons()
		}
		select {
		case err := <-done:
			if !errors.Is(err, iconcache.ErrUnavailable) {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("download was not cancelled")
		}
	}
	if _, err := s.SelectorIcon("request-3", "https://assets.example/icon.png"); !errors.Is(err, iconcache.ErrUnavailable) {
		t.Fatal("shutdown accepted a new image request")
	}
	if len(s.iconCancels) != 0 {
		t.Fatal("completed request retained its cancellation")
	}
}
