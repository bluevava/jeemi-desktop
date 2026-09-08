package geodata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type httpClientFunc func(*http.Request) (*http.Response, error)

func (function httpClientFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func geoResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		ContentLength: int64(len(body)),
		Body:          io.NopCloser(strings.NewReader(body)),
		Header:        make(http.Header),
	}
}

func TestOfficialDownloadRequiresPublisherDigest(t *testing.T) {
	contents := "validated geosite data"
	digest := sha256.Sum256([]byte(contents))
	expectedSHA := hex.EncodeToString(digest[:])
	preferences := DefaultPreferences()
	assetURL, err := preferences.URLFor(KindGeoSite)
	if err != nil {
		t.Fatal(err)
	}
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case assetURL:
			return geoResponse(http.StatusOK, contents), nil
		case assetURL + ".sha256sum":
			return geoResponse(http.StatusOK, expectedSHA+"  geosite.dat\n"), nil
		default:
			return geoResponse(http.StatusNotFound, ""), nil
		}
	})
	validator := &acceptingValidator{}
	manager, err := NewManager(ManagerOptions{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"),
		HTTPClient:    client,
		Validator:     validator,
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.Download(context.Background(), KindGeoSite, preferences, "mihomo", "v1")
	if err != nil {
		t.Fatal(err)
	}
	var installed AssetState
	for _, asset := range state.Assets {
		if asset.Kind == KindGeoSite {
			installed = asset
			break
		}
	}
	if !installed.Installed || !installed.PublisherVerified || installed.SHA256 != expectedSHA {
		t.Fatalf("official asset was not installed with publisher verification: %+v", installed)
	}
	if len(validator.calls) != 1 || validator.calls[0] != KindGeoSite {
		t.Fatalf("download did not use the selected core validator: %v", validator.calls)
	}
}

func TestOfficialDownloadRejectsDigestMismatch(t *testing.T) {
	preferences := DefaultPreferences()
	assetURL, err := preferences.URLFor(KindGeoSite)
	if err != nil {
		t.Fatal(err)
	}
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == assetURL+".sha256sum" {
			return geoResponse(http.StatusOK, strings.Repeat("a", 64)), nil
		}
		return geoResponse(http.StatusOK, "different contents"), nil
	})
	manager, err := NewManager(ManagerOptions{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"),
		HTTPClient:    client,
		Validator:     &acceptingValidator{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Download(context.Background(), KindGeoSite, preferences, "mihomo", "v1"); err == nil {
		t.Fatal("official asset with a mismatched publisher digest was accepted")
	}
}

func TestCustomDownloadWithoutSidecarKeepsLocalVerificationStatus(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.Source = SourceCustom
	preferences.CustomURLs.GeoSite = "https://example.test/geosite.dat"
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, ".sha256sum") {
			return geoResponse(http.StatusNotFound, ""), nil
		}
		return geoResponse(http.StatusOK, "custom geosite data"), nil
	})
	manager, err := NewManager(ManagerOptions{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"),
		HTTPClient:    client,
		Validator:     &acceptingValidator{},
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.Download(context.Background(), KindGeoSite, preferences, "mihomo", "v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range state.Assets {
		if asset.Kind == KindGeoSite && asset.PublisherVerified {
			t.Fatal("custom asset without a sidecar was marked as publisher verified")
		}
	}
}

func TestCheckOnlyRequestsCurrentlySelectedAssets(t *testing.T) {
	preferences := DefaultPreferences()
	var requested []string
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		requested = append(requested, request.URL.String())
		return geoResponse(http.StatusOK, strings.Repeat("b", 64)), nil
	})
	manager, err := NewManager(ManagerOptions{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"),
		HTTPClient:    client,
		Validator:     &acceptingValidator{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Check(context.Background(), preferences); err != nil {
		t.Fatal(err)
	}
	if len(requested) != 3 {
		t.Fatalf("expected three selected asset checks, got %d: %v", len(requested), requested)
	}
	for _, target := range requested {
		if strings.Contains(target, "/geoip.dat.sha256sum") {
			t.Fatalf("unused DAT asset was checked while MMDB mode was selected: %s", target)
		}
	}
}
