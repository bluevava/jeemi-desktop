package subscriptionformat

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net/url"
)

func parseCertificateURI(raw string, line int) (certificate, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment == "" {
		return certificate{}, failure("invalid_certificate", line, "")
	}
	pattern, err := normalizePattern(u.Host, false)
	if err != nil {
		return certificate{}, failure("invalid_certificate", line, "domain")
	}
	contents, ok := decodeBase64(u.Fragment)
	if !ok || len(contents) == 0 || len(contents) > maxCertificateBytes {
		return certificate{}, failure("invalid_certificate", line, "")
	}
	return certificate{Pattern: pattern, PEM: contents, Line: line}, nil
}

func applyCertificates(p *parsedDocument) error {
	for _, material := range p.Certificates {
		rest := bytes.TrimSpace(material.PEM)
		var leaf *x509.Certificate
		for len(rest) > 0 {
			if !bytes.HasPrefix(rest, []byte("-----BEGIN CERTIFICATE-----")) {
				return failure("invalid_certificate", material.Line, "")
			}
			block, remaining := pem.Decode(rest)
			if block == nil || block.Type != "CERTIFICATE" {
				return failure("invalid_certificate", material.Line, "")
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return failure("invalid_certificate", material.Line, "")
			}
			if leaf == nil {
				leaf = cert
			}
			rest = bytes.TrimSpace(remaining)
		}
		if leaf == nil || leaf.IsCA {
			return failure("unsupported_certificate", material.Line, "")
		}
		digest := sha256.Sum256(leaf.Raw)
		pin := hex.EncodeToString(digest[:])
		matched := false
		for i := range p.Nodes {
			n := &p.Nodes[i]
			name := n.TLS.ServerName
			if name == "" {
				name = n.Server
			}
			if !n.TLS.Enabled || n.TLS.RealityPublicKey != "" || !matchesDomain(material.Pattern, name) {
				continue
			}
			matched = true
			if leaf.VerifyHostname(name) != nil {
				return failure("certificate_name_mismatch", n.Line, "sni")
			}
			if n.TLS.LeafSHA256 != "" && n.TLS.LeafSHA256 != pin {
				return failure("certificate_pin_mismatch", n.Line, "certificate_pin")
			}
			n.TLS.LeafSHA256 = pin
		}
		if !matched {
			p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "unused_certificate", Line: material.Line})
		}
	}
	return nil
}
