// Package main provides a TLS fingerprint masquerading sidecar plugin for AIGateway.
//
// This plugin runs as an independent process. AGW connects to it via the
// simplified CONNECT protocol:
//
//	1. AGW dials 127.0.0.1:{PORT}
//	2. AGW sends: CONNECT target.host:443\r\n\r\n
//	3. Plugin establishes TCP connection to target
//	4. Plugin performs TLS handshake using utls (browser fingerprint masquerading)
//	5. Plugin replies: 200 OK\r\n\r\n
//	6. Plugin bidirectionally forwards data between AGW and the upstream
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	utls "github.com/refraction-networking/utls"
)

var profileMap = map[string]*utls.ClientHelloID{
	"chrome_120":  &utls.HelloChrome_120,
	"firefox_120": &utls.HelloFirefox_120,
	"safari_16":   &utls.HelloSafari_16_0,
	"edge_120":    &utls.HelloChrome_120,
	"ios_14":      &utls.HelloIOS_14,
}

var randomProfiles = []*utls.ClientHelloID{
	&utls.HelloChrome_120,
	&utls.HelloFirefox_120,
	&utls.HelloSafari_16_0,
	&utls.HelloIOS_14,
}

func main() {
	port, _ := strconv.Atoi(os.Getenv("PLUGIN_PORT"))
	if port == 0 {
		port = 9876
	}

	// Health check server on port+1
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		log.Printf("Health check on :%d/health", port+1)
		http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port+1), nil)
	}()

	// CONNECT proxy server
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Fatalf("Listen :%d: %v", port, err)
	}
	defer listener.Close()

	log.Printf("TLS fingerprint masquerade plugin on 127.0.0.1:%d", port)

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		listener.Close()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// Read CONNECT request line
	reader := bufio.NewReader(clientConn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		clientConn.Write([]byte("500 Invalid request\r\n\r\n"))
		return
	}

	targetAddr := strings.TrimSpace(strings.TrimPrefix(line, "CONNECT "))

	// Consume trailing \r\n after CONNECT line
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return
		}
		if b == '\n' {
			break
		}
	}

	// Pick TLS fingerprint profile
	profile := pickProfile("random")

	// Connect to target
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	targetConn, err := dialer.DialContext(context.Background(), "tcp", targetAddr)
	if err != nil {
		log.Printf("Dial %s error: %v", targetAddr, err)
		clientConn.Write([]byte("500 Connect failed\r\n\r\n"))
		return
	}

	// TLS handshake with utls
	host, _, _ := net.SplitHostPort(targetAddr)
	uconn := utls.UClient(targetConn, &utls.Config{
		ServerName: host,
	}, *profile)

	handshakeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := uconn.HandshakeContext(handshakeCtx); err != nil {
		log.Printf("utls handshake %s (%s): %v", targetAddr, profile.Str(), err)
		targetConn.Close()
		clientConn.Write([]byte("500 TLS handshake failed\r\n\r\n"))
		return
	}

	// Success → tell AGW
	clientConn.Write([]byte("200 OK\r\n\r\n"))
	log.Printf("Connected %s via %s", targetAddr, profile.Str())

	// Bidirectional relay
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		relay(uconn, clientConn)
		uconn.CloseWrite()
	}()

	go func() {
		defer wg.Done()
		relay(clientConn, uconn)
		if tc, ok := clientConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	wg.Wait()
	uconn.Close()
}

// relay copies data from src to dst
func relay(dst, src net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// pickProfile returns the utls ClientHelloID for the given profile name
func pickProfile(profile string) *utls.ClientHelloID {
	if profile == "" || profile == "random" {
		return randomProfiles[rand.Intn(len(randomProfiles))]
	}
	if id, ok := profileMap[profile]; ok {
		return id
	}
	return randomProfiles[rand.Intn(len(randomProfiles))]
}
