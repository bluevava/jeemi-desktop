package subscription

import (
	"encoding/base64"
	"net/http"
	"testing"
)

func TestParseResponseMetadataSupportsJeeyioCompatibleHeaders(t *testing.T) {
	title := "极光订阅"
	headers := http.Header{}
	headers.Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	headers.Set("Profile-Update-Interval", "24")
	headers.Set("Subscription-Userinfo", "upload=1073741824; download=4294967296; total=107374182400; expire=1893456000; name=fallback")

	metadata := parseResponseMetadata(headers)
	if metadata.SuggestedName != title {
		t.Fatalf("SuggestedName = %q, want %q", metadata.SuggestedName, title)
	}
	profile := metadata.Profile
	if !profile.HasTraffic || profile.UploadBytes != 1073741824 || profile.DownloadBytes != 4294967296 || profile.TotalBytes != 107374182400 {
		t.Fatalf("unexpected traffic profile: %+v", profile)
	}
	if profile.ExpiresAt != 1893456000 || profile.UpdateIntervalHours != 24 {
		t.Fatalf("unexpected expiry or update interval: %+v", profile)
	}
}

func TestParseResponseMetadataIgnoresMalformedOrUnsafeValues(t *testing.T) {
	headers := http.Header{}
	headers.Set("Profile-Title", "base64:not-valid")
	headers.Set("Profile-Update-Interval", "999999")
	headers.Set("Subscription-Userinfo", "upload=-1; download=999999999999999999; total=broken; expire=-2; name=bad\nname")

	metadata := parseResponseMetadata(headers)
	if metadata.SuggestedName != "" || metadata.Profile.hasObservedValues() {
		t.Fatalf("malformed response metadata was accepted: %+v", metadata)
	}
}
