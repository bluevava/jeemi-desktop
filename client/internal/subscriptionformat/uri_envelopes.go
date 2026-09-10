package subscriptionformat

import (
	"net/url"
	"strconv"
	"strings"
)

func expandCompatibleURI(raw, protocol string, line int) (string, error) {
	_, body, _ := strings.Cut(raw, "://")
	if protocol == "vmess" && !strings.ContainsAny(body, "@?#") {
		return expandVMess(body, line)
	}
	if protocol == "ssr" && !strings.ContainsAny(body, "@?#") {
		return expandSSR(body, line)
	}
	if protocol == "ss" {
		authority, fragment, hasFragment := strings.Cut(body, "#")
		authority, query, hasQuery := strings.Cut(authority, "?")
		if authority != "" && !strings.Contains(authority, "@") {
			decoded, ok := decodeBase64(strings.TrimSuffix(authority, "/"))
			if !ok {
				return raw, nil // Native server authority with query credentials.
			}
			at := strings.LastIndexByte(string(decoded), '@')
			if at < 0 {
				return raw, nil
			}
			cipher, password, ok := strings.Cut(string(decoded[:at]), ":")
			if !ok || !validCredential(password) {
				return "", failure("invalid_field", line, "credentials")
			}
			raw = "ss://" + url.UserPassword(cipher, password).String() + "@" + string(decoded[at+1:])
			if hasQuery {
				raw += "?" + query
			}
			if hasFragment {
				raw += "#" + fragment
			}
		}
	}
	return raw, nil
}

func expandVMess(body string, line int) (string, error) {
	decoded, ok := decodeBase64(body)
	if !ok {
		return "", failure("invalid_uri", line, "")
	}
	value, ok := parseNativeValue(string(decoded), nativeObject)
	if !ok {
		return "", failure("invalid_uri", line, "")
	}
	object := value.(map[string]any)
	q := url.Values{}
	keys := map[string]string{"ps": "name", "add": "server", "port": "port", "id": "uuid", "aid": "alterId", "scy": "cipher", "net": "network", "sni": "servername", "fp": "client-fingerprint", "alpn": "alpn", "path": "path", "host": "host"}
	for key, value := range object {
		var text string
		switch v := value.(type) {
		case string:
			text = v
		case int:
			text = strconv.Itoa(v)
		default:
			return "", failure("invalid_field", line, "node")
		}
		switch key {
		case "v":
			if text != "" && text != "2" {
				return "", failure("invalid_field", line, "version")
			}
		case "type":
			if text != "" && text != "none" {
				return "", failure("invalid_field", line, "transport")
			}
		case "tls":
			if text != "" && text != "none" && text != "tls" {
				return "", failure("invalid_field", line, "tls")
			}
			if text == "tls" {
				q.Set("tls", "true")
			}
		default:
			target, known := keys[key]
			if !known {
				return "", failure("invalid_field", line, "node")
			}
			if text != "" {
				q.Set(target, text)
			}
		}
	}
	return "vmess://?" + q.Encode(), nil
}

func expandSSR(body string, line int) (string, error) {
	decoded, ok := decodeBase64(body)
	if !ok {
		return "", failure("invalid_uri", line, "")
	}
	authority, query, _ := strings.Cut(string(decoded), "/?")
	parts := strings.Split(authority, ":")
	if len(parts) < 6 {
		return "", failure("invalid_uri", line, "")
	}
	start := len(parts) - 5
	password, ok := decodeBase64(parts[start+4])
	if !ok || !validCredential(string(password)) {
		return "", failure("invalid_field", line, "credentials")
	}
	q := url.Values{"server": {strings.Trim(strings.Join(parts[:start], ":"), "[]")}, "port": {parts[start]}, "protocol": {parts[start+1]}, "cipher": {parts[start+2]}, "obfs": {parts[start+3]}, "password": {string(password)}}
	options, err := url.ParseQuery(query)
	if err != nil {
		return "", failure("invalid_uri", line, "")
	}
	for key, values := range options {
		target := map[string]string{"obfsparam": "obfs-param", "protoparam": "protocol-param", "remarks": "name", "group": ""}
		name, known := target[key]
		if !known {
			return "", failure("invalid_field", line, "node")
		}
		for _, value := range values {
			text, ok := decodeBase64(value)
			if !ok || !validText(string(text), maxFieldBytes) {
				return "", failure("invalid_field", line, "node")
			}
			if name != "" {
				q.Add(name, string(text))
			}
		}
	}
	return "ssr://?" + q.Encode(), nil
}
