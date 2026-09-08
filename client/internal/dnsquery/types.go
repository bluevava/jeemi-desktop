// Package dnsquery implements bounded, one-shot DNS diagnostics. It does not
// provide a DNS listener, cache, or proxy protocol implementation.
package dnsquery

import (
	"context"
	"net"
)

type Request struct {
	ID          string `json:"id"`
	Domain      string `json:"domain"`
	ProxyDNS    string `json:"proxyDNS"`
	DirectDNS   string `json:"directDNS"`
	CustomDNS   string `json:"customDNS"`
	CustomProxy bool   `json:"customProxy"`
}

type FamilyResult struct {
	Type      string   `json:"type"`
	Addresses []string `json:"addresses"`
	Error     string   `json:"error"`
}

type Result struct {
	Server     string         `json:"server"`
	Route      string         `json:"route"`
	Selector   string         `json:"selector"`
	Node       string         `json:"node"`
	Status     string         `json:"status"`
	Error      string         `json:"error"`
	DurationMS int64          `json:"durationMS"`
	Records    []FamilyResult `json:"records"`
}

type Response struct {
	Domain    string `json:"domain"`
	QueriedAt string `json:"queriedAt"`
	Proxy     Result `json:"proxy"`
	Direct    Result `json:"direct"`
	Custom    Result `json:"custom"`
}

type DialContext func(context.Context, string, string) (net.Conn, error)

func Failed(server, route, code string) Result {
	return Result{Server: server, Route: route, Status: "error", Error: code, Records: []FamilyResult{}}
}
