//go:build windows

package daemon

import (
	"context"
	"testing"
	"time"
)

func TestCorePreparationCancelsAndJoinsBeforeRemoval(t *testing.T) {
	var gate preparationGate
	ctx, finish, err := gate.begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := gate.begin(); err == nil {
		t.Fatal("parallel verification accepted")
	}
	finished := make(chan struct{})
	go func() { <-ctx.Done(); finish(); close(finished) }()
	deadline, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := gate.close(deadline); err != nil {
		t.Fatal(err)
	}
	<-finished
	if _, _, err := gate.begin(); err == nil {
		t.Fatal("preparation accepted while closing")
	}
	gate.resume()
	_, finish, err = gate.begin()
	if err != nil {
		t.Fatal("failed cleanup could not resume", err)
	}
	finish()
}
