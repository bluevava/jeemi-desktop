package dnsquery

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const Timeout = 12 * time.Second

// Query asks both families independently, even when the local machine has no
// IPv6 route. Plain IP resolvers use DNS over TCP/53 so SOCKS and HTTP-only
// proxy nodes work without UDP support or TUN DNS interception.
func Query(ctx context.Context, domain, server, route string, dial DialContext) Result {
	started := time.Now()
	server, doh, err := Server(server)
	if err != nil {
		return Failed("", route, "invalid_server")
	}
	result := Result{Server: server, Route: route, Records: make([]FamilyResult, 2)}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if dial == nil {
		dial = (&net.Dialer{Timeout: Timeout}).DialContext
	}
	transport := &http.Transport{DialContext: dial, DisableKeepAlives: true, ForceAttemptHTTP2: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	exchange := func(ctx context.Context, packet []byte) ([]byte, error) {
		if doh {
			return exchangeHTTPS(ctx, client, server, packet)
		}
		return exchangeTCP(ctx, dial, net.JoinHostPort(server, "53"), packet)
	}
	var group sync.WaitGroup
	for i, family := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		group.Add(1)
		go func(i int, family dnsmessage.Type) {
			defer group.Done()
			result.Records[i] = queryFamily(ctx, domain, family, exchange)
		}(i, family)
	}
	group.Wait()
	result.DurationMS = time.Since(started).Milliseconds()
	result.Status = "success"
	failures := 0
	for _, record := range result.Records {
		if record.Error != "" {
			failures++
		}
	}
	if failures == 1 {
		result.Status = "partial"
	} else if failures == 2 {
		result.Status = "error"
	}
	return result
}

type exchangeFunc func(context.Context, []byte) ([]byte, error)

func queryFamily(ctx context.Context, domain string, family dnsmessage.Type, exchange exchangeFunc) FamilyResult {
	result := FamilyResult{Type: strings.TrimPrefix(family.String(), "Type"), Addresses: []string{}}
	visited := map[string]bool{}
	for depth := 0; depth < 12; depth++ {
		name, err := dnsmessage.NewName(domain + ".")
		if err != nil || visited[domain] {
			result.Error = "invalid_response"
			return result
		}
		visited[domain] = true
		var nonce [2]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			result.Error = "query_failed"
			return result
		}
		query := dnsmessage.Message{Header: dnsmessage.Header{ID: binary.BigEndian.Uint16(nonce[:]), RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: name, Type: family, Class: dnsmessage.ClassINET}}}
		packet, err := query.Pack()
		if err != nil {
			result.Error = "query_failed"
			return result
		}
		packet, err = exchange(ctx, packet)
		if err != nil {
			result.Error = errorCode(ctx, err)
			return result
		}
		var response dnsmessage.Message
		if response.Unpack(packet) != nil || !response.Response || response.ID != query.ID || response.OpCode != 0 || response.Truncated || len(response.Questions) != 1 || response.Questions[0].Type != family || response.Questions[0].Class != dnsmessage.ClassINET || !strings.EqualFold(response.Questions[0].Name.String(), name.String()) {
			result.Error = "invalid_response"
			return result
		}
		if response.RCode != dnsmessage.RCodeSuccess {
			result.Error = rcode(response.RCode)
			return result
		}
		// Follow only the question's CNAME chain; unrelated answer or glue IPs
		// must never be presented as addresses of the requested domain.
		current := strings.ToLower(name.String())
		aliases := map[string]bool{}
		for hops := 0; hops < 12; hops++ {
			if aliases[current] {
				result.Error = "invalid_response"
				return result
			}
			aliases[current] = true
			next := ""
			for _, rr := range response.Answers {
				if rr.Header.Class != dnsmessage.ClassINET || !strings.EqualFold(rr.Header.Name.String(), current) {
					continue
				}
				var ip string
				switch body := rr.Body.(type) {
				case *dnsmessage.AResource:
					if family == dnsmessage.TypeA {
						ip = net.IP(body.A[:]).String()
					}
				case *dnsmessage.AAAAResource:
					if family == dnsmessage.TypeAAAA {
						ip = net.IP(body.AAAA[:]).String()
					}
				case *dnsmessage.CNAMEResource:
					next = strings.ToLower(body.CNAME.String())
				}
				if ip != "" && !slices.Contains(result.Addresses, ip) {
					result.Addresses = append(result.Addresses, ip)
				}
			}
			if len(result.Addresses) > 0 {
				return result
			}
			if next == "" {
				break
			}
			current = next
		}
		if strings.EqualFold(current, name.String()) {
			return result
		}
		domain = strings.TrimSuffix(current, ".")
	}
	result.Error = "invalid_response"
	return result
}

func exchangeTCP(ctx context.Context, dial DialContext, address string, packet []byte) ([]byte, error) {
	conn, err := dial(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	framed := make([]byte, len(packet)+2)
	binary.BigEndian.PutUint16(framed, uint16(len(packet)))
	copy(framed[2:], packet)
	if _, err := io.Copy(conn, bytes.NewReader(framed)); err != nil {
		return nil, err
	}
	var length [2]byte
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return nil, err
	}
	response := make([]byte, binary.BigEndian.Uint16(length[:]))
	_, err = io.ReadFull(conn, response)
	return response, err
}

func exchangeHTTPS(ctx context.Context, client *http.Client, server string, packet []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server, bytes.NewReader(packet))
	if err != nil {
		return nil, errors.New("invalid_server")
	}
	request.Header.Set("Content-Type", "application/dns-message")
	request.Header.Set("Accept", "application/dns-message")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || contentType != "application/dns-message" {
		return nil, errors.New("invalid_response")
	}
	packet, err = io.ReadAll(io.LimitReader(response.Body, 65536))
	if len(packet) > 65535 {
		return nil, errors.New("invalid_response")
	}
	return packet, err
}

func errorCode(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) {
		return "cancelled"
	}
	var networkError net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return "timeout"
	}
	if err.Error() == "invalid_response" {
		return "invalid_response"
	}
	return "query_failed"
}

func rcode(code dnsmessage.RCode) string {
	switch code {
	case dnsmessage.RCodeNameError:
		return "nxdomain"
	case dnsmessage.RCodeServerFailure:
		return "servfail"
	case dnsmessage.RCodeRefused:
		return "refused"
	default:
		return "dns_error"
	}
}
