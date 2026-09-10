package subscriptionformat

import (
	"encoding/hex"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Definitions bind aliases to a semantic field. The protocol's reader opts in
// to each definition: an alias valid for one protocol is not global passthrough.
type fieldDefinition struct {
	aliases []string
	kind    string
}

var fields = map[string]fieldDefinition{
	"sni":                {[]string{"sni", "servername", "peer"}, "server_name"},
	"certificate_pin":    {[]string{"fingerprint", "server-cert-fingerprint-sha256", "pcs"}, "pin"},
	"client_fingerprint": {[]string{"fp", "client-fingerprint"}, "token"},
	"skip_cert_verify":   {[]string{"skip-cert-verify", "allowInsecure", "insecure", "allow_insecure"}, "bool"},
	"alpn":               {[]string{"alpn"}, "list"},
	"udp":                {[]string{"udp", "udp-relay"}, "bool"},
	"tcp_fast_open":      {[]string{"tfo"}, "bool"},
	"idle_check":         {[]string{"idle_session_check_interval", "idle-session-check-interval"}, "seconds"},
	"idle_timeout":       {[]string{"idle_session_timeout", "idle-session-timeout"}, "seconds"},
	"min_idle":           {[]string{"min_idle_session", "min-idle-session"}, "integer"},
	"congestion":         {[]string{"congestion_control", "congestion-controller"}, "token"},
	"udp_relay_mode":     {[]string{"udp_relay_mode", "udp-relay-mode"}, "token"},
	"reduce_rtt":         {[]string{"zero_rtt_handshake", "reduce-rtt"}, "bool"},
	"heartbeat":          {[]string{"heartbeat"}, "milliseconds"},
	"security":           {[]string{"security"}, "token"},
	"network":            {[]string{"type", "network"}, "token"},
	"path":               {[]string{"path"}, "text"},
	"host":               {[]string{"host"}, "text"},
	"service_name":       {[]string{"serviceName", "service-name"}, "text"},
	"mode":               {[]string{"mode"}, "token"},
	"flow":               {[]string{"flow"}, "token"},
	"encryption":         {[]string{"encryption"}, "text"},
	"packet_encoding":    {[]string{"packetEncoding", "packet-encoding"}, "token"},
	"reality_public_key": {[]string{"pbk", "public-key"}, "text"},
	"reality_short_id":   {[]string{"sid", "short-id"}, "text"},
	"psk":                {[]string{"psk"}, "text"},
	"version":            {[]string{"version"}, "integer"},
	"userkey":            {[]string{"userkey"}, "text"},
	"obfs":               {[]string{"obfs", "obfs-mode"}, "token"},
	"obfs_host":          {[]string{"obfs_host", "obfs-host"}, "text"},
}

type fieldReader struct {
	values url.Values
	used   map[string]bool
	line   int
	err    error
}

func reader(values url.Values, line int) *fieldReader {
	return &fieldReader{values: values, used: map[string]bool{}, line: line}
}

func (r *fieldReader) read(id string) any {
	d := fields[id]
	var result any
	for _, key := range d.aliases {
		for _, value := range r.values[key] {
			r.used[key] = true
			normal, ok := normalizeField(value, d.kind)
			if !ok {
				r.err = failure("invalid_field", r.line, id)
				return nil
			}
			if result != nil && !reflect.DeepEqual(result, normal) {
				r.err = failure("conflicting_alias", r.line, id)
				return nil
			}
			result = normal
		}
	}
	return result
}
func (r *fieldReader) text(id string) string { v, _ := r.read(id).(string); return v }
func (r *fieldReader) boolean(id string) *bool {
	v := r.read(id)
	if v == nil {
		return nil
	}
	b := v.(bool)
	return &b
}
func (r *fieldReader) number(id string) *int64 {
	v := r.read(id)
	if v == nil {
		return nil
	}
	n := v.(int64)
	return &n
}
func (r *fieldReader) list(id string) []string { v, _ := r.read(id).([]string); return v }
func (r *fieldReader) unknown() bool {
	for key := range r.values {
		if !r.used[key] {
			return true
		}
	}
	return false
}

func normalizeField(value, kind string) (any, bool) {
	if !validText(value, maxFieldBytes) {
		return nil, false
	}
	switch kind {
	case "server_name":
		if value == "" {
			return "", true
		}
		normalized, err := normalizeServer(value)
		return normalized, err == nil
	case "text":
		return value, true
	case "token":
		v := strings.TrimSpace(value)
		return v, !strings.ContainsAny(v, " \t")
	case "bool":
		switch strings.ToLower(value) {
		case "true", "1":
			return true, true
		case "false", "0":
			return false, true
		}
		return nil, false
	case "pin":
		v := strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "sha256:"), ":", "")
		b, e := hex.DecodeString(v)
		return v, e == nil && len(b) == 32
	case "list":
		items := []string{}
		for _, v := range strings.Split(value, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				return nil, false
			}
			items = appendUnique(items, v)
		}
		return items, true
	case "integer":
		n, e := strconv.ParseInt(value, 10, 32)
		return n, e == nil && n >= 0
	case "seconds", "milliseconds":
		unit := time.Second
		if kind == "milliseconds" {
			unit = time.Millisecond
		}
		d, e := time.ParseDuration(value)
		// The hyphenated AnyTLS fields use integer seconds. URI duration
		// aliases may also use seconds, but heartbeat always has a unit.
		if e != nil && kind == "seconds" {
			n, err := strconv.ParseInt(value, 10, 32)
			return n, err == nil && n > 0 && n <= 86400
		}
		return int64(d / unit), e == nil && d > 0 && d%unit == 0 && d <= 24*time.Hour
	}
	return nil, false
}

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}
