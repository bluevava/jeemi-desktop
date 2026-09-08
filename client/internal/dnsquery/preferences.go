package dnsquery

import (
	"errors"
	"strings"
	"unicode"
)

type Preferences struct {
	CustomDNS string `json:"customDNS"`
}

func DefaultPreferences() Preferences {
	return Preferences{CustomDNS: "https://dns.google/dns-query"}
}

// PreparePreferences remembers the submitted field, even when the lookup
// fails or the address needs correcting. Endpoint validation stays per card.
func PreparePreferences(input Preferences) (Preferences, error) {
	input.CustomDNS = strings.TrimSpace(input.CustomDNS)
	if len(input.CustomDNS) > 2048 || strings.IndexFunc(input.CustomDNS, unicode.IsControl) >= 0 {
		return Preferences{}, errors.New("invalid_server")
	}
	return input, nil
}
