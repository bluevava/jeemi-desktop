package subscription

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxJavaScriptSafeInteger int64 = 9_007_199_254_740_991

type responseMetadata struct {
	SuggestedName string
	Profile       RemoteProfile
}

func parseResponseMetadata(headers http.Header) responseMetadata {
	profileTitle := decodeHeaderText(headers.Get("Profile-Title"))
	userinfoName, profile := parseSubscriptionUserinfo(headers.Get("Subscription-Userinfo"))
	if interval, ok := parseBoundedInteger(headers.Get("Profile-Update-Interval"), 1, 24*365); ok {
		profile.UpdateIntervalHours = int(interval)
	}
	suggestedName := profileTitle
	if suggestedName == "" {
		suggestedName = userinfoName
	}
	return responseMetadata{SuggestedName: suggestedName, Profile: profile}
}

func parseSubscriptionUserinfo(value string) (string, RemoteProfile) {
	var profile RemoteProfile
	var name string
	for _, item := range strings.Split(value, ";") {
		key, raw, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		raw = strings.TrimSpace(raw)
		switch key {
		case "upload":
			if parsed, ok := parseBoundedInteger(raw, 0, maxJavaScriptSafeInteger); ok {
				profile.UploadBytes = parsed
				profile.HasTraffic = true
			}
		case "download":
			if parsed, ok := parseBoundedInteger(raw, 0, maxJavaScriptSafeInteger); ok {
				profile.DownloadBytes = parsed
				profile.HasTraffic = true
			}
		case "total":
			if parsed, ok := parseBoundedInteger(raw, 0, maxJavaScriptSafeInteger); ok {
				profile.TotalBytes = parsed
				profile.HasTraffic = true
			}
		case "expire":
			if parsed, ok := parseBoundedInteger(raw, 1, maxJavaScriptSafeInteger); ok {
				profile.ExpiresAt = parsed
			}
		case "name":
			name = decodeHeaderText(raw)
		}
	}
	return name, profile
}

func parseBoundedInteger(value string, minimum, maximum int64) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, false
	}
	return parsed, true
}

func decodeHeaderText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "base64:") {
		encoded := strings.TrimSpace(value[len("base64:"):])
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		}
		if err != nil || !utf8.Valid(decoded) {
			return ""
		}
		value = string(decoded)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return ""
		}
	}
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxSubscriptionNameLength {
		runes = runes[:maxSubscriptionNameLength]
	}
	return string(runes)
}

func (profile RemoteProfile) hasObservedValues() bool {
	return profile.HasTraffic || profile.ExpiresAt > 0 || profile.UpdateIntervalHours > 0
}
