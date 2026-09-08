package dnsquery

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestInputValidation(t *testing.T) {
	for _, input := range []string{"example.com:53", "1.2.3.4", "::1", "*.example.com", "a..com", "-a.com"} {
		if _, err := Domain(input); err == nil {
			t.Errorf("accepted domain %q", input)
		}
	}
	if domain, err := Domain("  例子.测试. "); err != nil || domain != "xn--fsqu00a.xn--0zwm56d" {
		t.Fatalf("IDN: %q %v", domain, err)
	}
	for _, input := range []string{"223.5.5.5", "2606:4700:4700::1111", "https://dns.example/dns-query?key=private"} {
		if _, _, err := Server(input); err != nil {
			t.Errorf("rejected server %q", input)
		}
	}
	for _, input := range []string{"localhost", "http://dns.example/dns-query", "https://a:b@dns.example/", "https://dns.example/#DIRECT", "https://dns.example:99999/", "https://dns.example:abc/", "1.2.3.4:53", "0.0.0.0", "fe80::1%eth0"} {
		if _, _, err := Server(input); err == nil {
			t.Errorf("accepted server %q", input)
		}
	}
}

func TestQueriesBothFamiliesAndKeepsAllAnswersInCNAMEChain(t *testing.T) {
	var count atomic.Int32
	dial := pipeResolver(t, func(query dnsmessage.Message) dnsmessage.Message {
		count.Add(1)
		response := answerFor(query)
		alias, _ := dnsmessage.NewName("target.example.")
		unrelated, _ := dnsmessage.NewName("unrelated.example.")
		response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: query.Questions[0].Name, Type: dnsmessage.TypeCNAME, Class: dnsmessage.ClassINET}, Body: &dnsmessage.CNAMEResource{CNAME: alias}})
		if query.Questions[0].Type == dnsmessage.TypeA {
			for _, ip := range [][4]byte{{192, 0, 2, 1}, {192, 0, 2, 2}, {192, 0, 2, 1}} {
				response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: alias, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}, Body: &dnsmessage.AResource{A: ip}})
			}
			response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: unrelated, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}, Body: &dnsmessage.AResource{A: [4]byte{192, 0, 2, 99}}})
		} else {
			response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: alias, Type: dnsmessage.TypeAAAA, Class: dnsmessage.ClassINET}, Body: &dnsmessage.AAAAResource{AAAA: [16]byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}})
		}
		return response
	})
	result := Query(context.Background(), "example.com", "192.0.2.53", "direct", dial)
	if count.Load() != 2 || result.Status != "success" || len(result.Records[0].Addresses) != 2 || result.Records[1].Addresses[0] != "2001:db8::1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestCNAMEMissingAddressTriggersFollowupAndEmptyAAAAIsSuccess(t *testing.T) {
	dial := pipeResolver(t, func(query dnsmessage.Message) dnsmessage.Message {
		response := answerFor(query)
		if query.Questions[0].Type == dnsmessage.TypeAAAA {
			return response
		}
		if query.Questions[0].Name.String() == "example.com." {
			alias, _ := dnsmessage.NewName("alias.example.")
			response.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: query.Questions[0].Name, Type: dnsmessage.TypeCNAME, Class: dnsmessage.ClassINET}, Body: &dnsmessage.CNAMEResource{CNAME: alias}}}
		} else {
			response.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: query.Questions[0].Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}, Body: &dnsmessage.AResource{A: [4]byte{192, 0, 2, 5}}}}
		}
		return response
	})
	result := Query(context.Background(), "example.com", "192.0.2.53", "direct", dial)
	if result.Status != "success" || len(result.Records[0].Addresses) != 1 || len(result.Records[1].Addresses) != 0 || result.Records[1].Error != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestPartialFailureAndInvalidReplies(t *testing.T) {
	for _, failure := range []string{"rcode", "id", "question", "truncated"} {
		t.Run(failure, func(t *testing.T) {
			dial := pipeResolver(t, func(query dnsmessage.Message) dnsmessage.Message {
				response := answerFor(query)
				if query.Questions[0].Type == dnsmessage.TypeA {
					return response
				}
				switch failure {
				case "rcode":
					response.RCode = dnsmessage.RCodeNameError
				case "id":
					response.ID++
				case "question":
					response.Questions = nil
				case "truncated":
					response.Truncated = true
				}
				return response
			})
			result := Query(context.Background(), "example.com", "192.0.2.53", "proxy", dial)
			if result.Status != "partial" || result.Records[0].Error != "" || result.Records[1].Error == "" {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestCancellationClosesDNSConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	connected := make(chan struct{}, 2)
	closed := make(chan struct{}, 2)
	dial := func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			connected <- struct{}{}
			_, _ = io.Copy(io.Discard, server)
			closed <- struct{}{}
		}()
		return client, nil
	}
	done := make(chan Result, 1)
	go func() { done <- Query(ctx, "example.com", "192.0.2.53", "direct", dial) }()
	<-connected
	cancel()
	select {
	case result := <-done:
		if result.Records[0].Error != "cancelled" || result.Records[1].Error != "cancelled" {
			t.Fatalf("result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("query did not cancel")
	}
	for i := 0; i < 2; i++ {
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("connection leaked")
		}
	}
}

func TestHTTPSWireFormatAndNoRedirects(t *testing.T) {
	var redirect atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirect.Load() {
			http.Redirect(w, r, "/other", http.StatusFound)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/dns-message" || r.Header.Get("Accept") != "application/dns-message" {
			t.Error("invalid DoH request")
		}
		packet, _ := io.ReadAll(r.Body)
		var query dnsmessage.Message
		_ = query.Unpack(packet)
		response := answerFor(query)
		packet, _ = response.Pack()
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(packet)
	}))
	defer server.Close()
	name, _ := dnsmessage.NewName("example.com.")
	packet, _ := (&dnsmessage.Message{Questions: []dnsmessage.Question{{Name: name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}).Pack()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if _, err := exchangeHTTPS(context.Background(), client, server.URL, packet); err != nil {
		t.Fatal(err)
	}
	redirect.Store(true)
	if _, err := exchangeHTTPS(context.Background(), client, server.URL, packet); err == nil {
		t.Fatal("accepted redirect")
	}
	// The real query transport verifies certificates and must reject this
	// test-only CA. Never include the private endpoint token in the result.
	result := Query(context.Background(), "example.com", server.URL+"/?token=private", "direct", nil)
	if result.Status != "error" || strings.Contains(result.Records[0].Error, "private") {
		t.Fatalf("result = %+v", result)
	}
}

func answerFor(query dnsmessage.Message) dnsmessage.Message {
	return dnsmessage.Message{Header: dnsmessage.Header{ID: query.ID, Response: true, RecursionAvailable: true}, Questions: query.Questions}
}

func pipeResolver(t *testing.T, respond func(dnsmessage.Message) dnsmessage.Message) DialContext {
	t.Helper()
	return func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "192.0.2.53:53" {
			t.Errorf("unexpected DNS route %s %s", network, address)
		}
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			var prefix [2]byte
			if _, err := io.ReadFull(server, prefix[:]); err != nil {
				return
			}
			packet := make([]byte, binary.BigEndian.Uint16(prefix[:]))
			if _, err := io.ReadFull(server, packet); err != nil {
				return
			}
			var query dnsmessage.Message
			if err := query.Unpack(packet); err != nil {
				t.Error(err)
				return
			}
			response := respond(query)
			packet, err := response.Pack()
			if err != nil {
				t.Error(err)
				return
			}
			binary.BigEndian.PutUint16(prefix[:], uint16(len(packet)))
			_, _ = server.Write(append(prefix[:], packet...))
		}()
		return client, nil
	}
}
