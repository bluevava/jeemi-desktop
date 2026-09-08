package daemon

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"jeemi/internal/platform/macnetwork"
)

type dnsGateway struct {
	udp         *net.UDPConn
	tcp         net.Listener
	target      string
	once        sync.Once
	semaphore   chan struct{}
	mu          sync.Mutex
	closed      bool
	connections map[net.Conn]bool
}

// macOS DNS preferences contain IPs, not ports. Forward only loopback DNS to
// mihomo's selected listener; resolution and filtering remain in mihomo.
func openDNS(target string) (DNSGateway, string, error) {
	host, port, err := net.SplitHostPort(target)
	number, parseErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || parseErr != nil || ip == nil || (!ip.IsLoopback() && !ip.IsUnspecified()) || number < 1 || number > 65535 {
		return nil, "", macnetwork.Failure("dns_invalid")
	}
	if ip.IsUnspecified() {
		host = "127.0.0.1"
		if ip.To4() == nil {
			host = "::1"
		}
	}
	if number == 53 {
		return nil, host, nil
	}
	target = net.JoinHostPort(host, port)
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53})
	if err != nil {
		return nil, "", macnetwork.Failure("dns_in_use")
	}
	tcp, err := net.Listen("tcp4", "127.0.0.1:53")
	if err != nil {
		udp.Close()
		return nil, "", macnetwork.Failure("dns_in_use")
	}
	g := &dnsGateway{udp: udp, tcp: tcp, target: target, semaphore: make(chan struct{}, 64), connections: map[net.Conn]bool{}}
	go g.serveUDP()
	go g.serveTCP()
	return g, "127.0.0.1", nil
}
func (g *dnsGateway) track(connection net.Conn) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		connection.Close()
		return false
	}
	g.connections[connection] = true
	return true
}
func (g *dnsGateway) release(connection net.Conn) {
	connection.Close()
	g.mu.Lock()
	delete(g.connections, connection)
	g.mu.Unlock()
}
func (g *dnsGateway) Close() error {
	g.once.Do(func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.closed = true
		g.udp.Close()
		g.tcp.Close()
		for connection := range g.connections {
			connection.Close()
		}
	})
	return nil
}
func (g *dnsGateway) serveUDP() {
	buffer := make([]byte, 65535)
	for {
		n, peer, err := g.udp.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		if n < 12 {
			continue
		}
		select {
		case g.semaphore <- struct{}{}:
		default:
			continue
		}
		packet := append([]byte(nil), buffer[:n]...)
		go func() {
			defer func() { <-g.semaphore }()
			connection, err := net.DialTimeout("udp", g.target, 2*time.Second)
			if err != nil {
				return
			}
			if !g.track(connection) {
				return
			}
			defer g.release(connection)
			_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err = connection.Write(packet); err != nil {
				return
			}
			response := make([]byte, 65535)
			n, err := connection.Read(response)
			if err == nil && n >= 12 && response[0] == packet[0] && response[1] == packet[1] {
				_, _ = g.udp.WriteToUDP(response[:n], peer)
			}
		}()
	}
}
func (g *dnsGateway) serveTCP() {
	for {
		client, err := g.tcp.Accept()
		if err != nil {
			return
		}
		select {
		case g.semaphore <- struct{}{}:
		default:
			client.Close()
			continue
		}
		go func() {
			defer func() { g.release(client); <-g.semaphore }()
			if !g.track(client) {
				return
			}
			remote, err := net.DialTimeout("tcp", g.target, 2*time.Second)
			if err != nil {
				return
			}
			if !g.track(remote) {
				return
			}
			defer g.release(remote)
			for i := 0; i < 16; i++ {
				deadline := time.Now().Add(5 * time.Second)
				_ = client.SetDeadline(deadline)
				_ = remote.SetDeadline(deadline)
				if forwardDNSMessage(remote, client) != nil || forwardDNSMessage(client, remote) != nil {
					return
				}
			}
		}()
	}
}
func forwardDNSMessage(destination io.Writer, source io.Reader) error {
	var size [2]byte
	if _, err := io.ReadFull(source, size[:]); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint16(size[:]))
	if n < 12 {
		return io.ErrUnexpectedEOF
	}
	buffer := make([]byte, n+2)
	copy(buffer, size[:])
	if _, err := io.ReadFull(source, buffer[2:]); err != nil {
		return err
	}
	_, err := io.Copy(destination, bytes.NewReader(buffer))
	return err
}
