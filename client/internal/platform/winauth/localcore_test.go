package winauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func coreFixture() ([]byte, Target) {
	data := append([]byte("MZ"), bytes.Repeat([]byte{42}, 1022)...)
	sum := sha256.Sum256(data)
	return data, Target{Version: "unknown-0123456789abcdef", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), ExecutablePath: `C:\Users\test\jeemi_data\core\mihomo\unknown-0123456789abcdef\windows-amd64\mihomo.exe`}
}

func TestLocalCoreAcceptsImportedVersionAndVerifiesRecordedHash(t *testing.T) {
	data, target := coreFixture()
	if err := VerifyCore(context.Background(), bytes.NewReader(data), target); err != nil {
		t.Fatal(err)
	}
	data[len(data)-1]++
	if err := VerifyCore(context.Background(), bytes.NewReader(data), target); err == nil || !strings.Contains(err.Error(), "core_changed") {
		t.Fatalf("changed file was not rejected: %v", err)
	}
	if err := VerifyCore(context.Background(), bytes.NewReader(data[:64]), target); err == nil {
		t.Fatal("truncated core accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyCore(ctx, bytes.NewReader(data), target); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestCoreCheckProtocolCarriesInstalledPathWithoutBinaryTransfer(t *testing.T) {
	_, target := coreFixture()
	request := Request{Operation: "check", Target: &target}
	data, _ := json.Marshal(request)
	if _, err := decodeRequest(data); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Request){
		func(r *Request) { r.Target.ExecutablePath = "" },
		func(r *Request) { r.Target.Size = 0 },
		func(r *Request) { r.Operation = "exec" },
		func(r *Request) { r.SourceRoot = "C:/Windows" },
		func(r *Request) { r.Target.Version = "../core" },
	} {
		copyTarget := target
		candidate := request
		candidate.Target = &copyTarget
		mutate(&candidate)
		data, _ := json.Marshal(candidate)
		if _, err := decodeRequest(data); err == nil {
			t.Fatalf("invalid core check accepted: %s", data)
		}
	}
}
