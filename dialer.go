// Package yamuxdialer provides a configurable yamux dialer that works as a drop-in
// replacement for net.Dialer, allowing HTTP/HTTPS clients to route traffic through
// a yamux proxy server.
//
// The dialer maintains a single persistent yamux session to the proxy and opens
// new streams for each connection request, making it efficient for high-throughput
// scenarios while supporting connection rotation.
//
// Basic usage with http.Client:
//
//	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
//	dialer, _ := yamuxdialer.New(config)
//	defer dialer.Close()
//
//	client := &http.Client{
//		Transport: &http.Transport{
//			DialContext:       dialer.DialContext,
//			DisableKeepAlives: true, // REQUIRED for proper stream rotation
//		},
//	}
//
//	resp, _ := client.Get("https://example.com")
//
// IMPORTANT: When using with http.Client, you MUST set DisableKeepAlives: true
// in the http.Transport. This ensures each HTTP request opens a new yamux stream,
// which is critical for rotating proxy services where each stream represents a
// different exit IP address.
package yamuxdialer

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// Dialer is a configurable yamux dialer that can be used as a drop-in replacement
// for regular net.Dialer. It maintains a yamux session to a proxy server and
// opens streams for each dial request.
//
// Usage with http.Client:
//
//	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
//	dialer, _ := yamuxdialer.New(config)
//	defer dialer.Close()
//
//	client := &http.Client{
//		Transport: &http.Transport{
//			DialContext:       dialer.DialContext,
//			DisableKeepAlives: true, // REQUIRED: Ensures each request opens a new yamux stream
//		},
//	}
//
//	resp, _ := client.Get("https://example.com")
//
// IMPORTANT: When using with http.Client, you MUST set DisableKeepAlives: true
// in the http.Transport. Without this, the HTTP client will reuse the same yamux
// stream for multiple requests, which prevents proper connection rotation and
// can cause issues with rotating proxy services.
//
// The dialer implements the same interface as net.Dialer, so it can be used
// anywhere a net.Dialer is expected (e.g., http.Transport.DialContext).
type Dialer struct {
	config  *Config
	mu      sync.RWMutex
	session *yamux.Session
	conn    net.Conn
}

// New creates a new yamux dialer with the given configuration.
//
// The dialer maintains a single persistent yamux session to the proxy server
// and opens new streams for each Dial/DialContext call. This is efficient
// for high-throughput scenarios as it avoids establishing a new TCP connection
// for each request.
//
// Example:
//
//	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
//	dialer, err := yamuxdialer.New(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer dialer.Close()
func New(config *Config) (*Dialer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.ProxyAddress == "" {
		return nil, fmt.Errorf("proxy address is required")
	}

	d := &Dialer{
		config: config,
	}

	return d, nil
}

// Dial connects to the given network and address through the yamux proxy.
// This is equivalent to calling DialContext with a background context.
//
// The network should be "tcp", "tcp4", or "tcp6".
// The address is the target address to connect to (e.g., "example.com:443").
//
// Each call to Dial opens a new yamux stream on the underlying session.
// The returned connection can be used like any net.Conn.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext connects to the given network and address through the yamux proxy
// with the given context. This method is compatible with http.Transport.DialContext.
//
// The network should be "tcp", "tcp4", or "tcp6".
// The address is the target address to connect to (e.g., "example.com:443").
//
// Each call opens a new yamux stream on the underlying session. The connection
// is established by sending an HTTP CONNECT request through the yamux stream.
// Per-request credentials can be set with WithProxyBasicAuth on ctx; otherwise
// Config.ProxyUsername / Config.ProxyPassword are used. Credentials are included
// in each CONNECT request (they do not create a separate yamux session).
//
// The context can be used to cancel the dial operation or set timeouts.
// The returned connection implements net.Conn and can be used for any protocol
// (HTTP, HTTPS, raw TCP, etc.).
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported network: %s (only tcp, tcp4, tcp6 are supported)", network)
	}

	// Get or create yamux session
	session, err := d.getOrCreateSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get yamux session: %w", err)
	}

	// Open a new stream on the session with timeout handling
	var stream net.Conn
	streamErr := make(chan error, 1)
	streamChan := make(chan net.Conn, 1)

	go func() {
		s, err := session.Open()
		if err != nil {
			streamErr <- err
			return
		}
		streamChan <- s
	}()

	// Wait for stream with timeout
	streamCtx := ctx
	if d.config.StreamOpenTimeout > 0 {
		var cancel context.CancelFunc
		streamCtx, cancel = context.WithTimeout(ctx, d.config.StreamOpenTimeout)
		defer cancel()
	}

	select {
	case stream = <-streamChan:
		// Success
	case err := <-streamErr:
		// If session is closed, try to reconnect
		if session.IsClosed() {
			d.mu.Lock()
			if d.session == session {
				d.session = nil
				if d.conn != nil {
					d.conn.Close()
					d.conn = nil
				}
			}
			d.mu.Unlock()
			// Retry once with a new session
			session, err = d.getOrCreateSession(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to reconnect yamux session: %w", err)
			}
			// Retry opening stream
			go func() {
				s, err := session.Open()
				if err != nil {
					streamErr <- err
					return
				}
				streamChan <- s
			}()
			select {
			case stream = <-streamChan:
				// Success
			case err := <-streamErr:
				return nil, fmt.Errorf("failed to open stream: %w", err)
			case <-streamCtx.Done():
				return nil, fmt.Errorf("failed to open stream: %w", streamCtx.Err())
			}
		} else {
			return nil, fmt.Errorf("failed to open stream: %w", err)
		}
	case <-streamCtx.Done():
		return nil, fmt.Errorf("failed to open stream: timeout")
	}

	// Build CONNECT request with optional authentication
	connectMsg := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", address)
	connectMsg += fmt.Sprintf("Host: %s\r\n", address)

	username := d.config.ProxyUsername
	password := d.config.ProxyPassword
	if u, p, ok := proxyAuthFromContext(ctx); ok {
		username, password = u, p
	}
	if username != "" || password != "" {
		auth := username + ":" + password
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		connectMsg += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", encodedAuth)
	}

	connectMsg += "\r\n"
	if d.config.ConnectionWriteTimeout > 0 {
		stream.SetWriteDeadline(time.Now().Add(d.config.ConnectionWriteTimeout))
	}
	_, err = stream.Write([]byte(connectMsg))
	if err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %w", err)
	}

	// Read and parse the CONNECT response
	// Set read deadline for response
	if d.config.ConnectionWriteTimeout > 0 {
		stream.SetReadDeadline(time.Now().Add(d.config.ConnectionWriteTimeout))
	}

	reader := bufio.NewReader(stream)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	resp.Body.Close()

	// Check if connection was established
	if resp.StatusCode != http.StatusOK {
		stream.Close()
		return nil, fmt.Errorf("proxy returned error: %s", resp.Status)
	}

	// Reset deadlines
	stream.SetWriteDeadline(time.Time{})
	stream.SetReadDeadline(time.Time{})

	return stream, nil
}

// getOrCreateSession returns an existing yamux session or creates a new one
func (d *Dialer) getOrCreateSession(ctx context.Context) (*yamux.Session, error) {
	d.mu.RLock()
	session := d.session
	d.mu.RUnlock()

	if session != nil && !session.IsClosed() {
		return session, nil
	}

	// Need to create a new session
	d.mu.Lock()
	defer d.mu.Unlock()

	// Double-check after acquiring write lock
	if d.session != nil && !d.session.IsClosed() {
		return d.session, nil
	}

	// Close old connection if it exists
	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}

	// Dial the proxy server
	dialer := &net.Dialer{
		Timeout: d.config.DialTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", d.config.ProxyAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to dial proxy: %w", err)
	}

	// Create yamux config
	yamuxConfig := yamux.DefaultConfig()
	if d.config.KeepAliveInterval > 0 {
		yamuxConfig.KeepAliveInterval = d.config.KeepAliveInterval
	}
	if d.config.ConnectionWriteTimeout > 0 {
		yamuxConfig.ConnectionWriteTimeout = d.config.ConnectionWriteTimeout
	}
	// Zero must keep yamux.DefaultConfig(); setting 0 explicitly would fail
	// VerifyConfig (MaxStreamWindowSize must be >= yamux initial window 256KiB).
	if d.config.MaxStreamWindowSize > 0 {
		yamuxConfig.MaxStreamWindowSize = d.config.MaxStreamWindowSize
	}

	// yamux requires either Logger or LogOutput to be set
	// Use io.Discard as default to silently discard logs if no LogOutput is provided
	if d.config.LogOutput != nil {
		yamuxConfig.LogOutput = d.config.LogOutput
	} else {
		yamuxConfig.LogOutput = io.Discard
	}

	// Create yamux session
	session, err = yamux.Client(conn, yamuxConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create yamux session: %w", err)
	}

	d.session = session
	d.conn = conn

	return session, nil
}

// Close closes the yamux session and underlying TCP connection to the proxy.
// After closing, the dialer cannot be used for new connections. Any existing
// streams will be closed when the session is closed.
//
// It is safe to call Close multiple times.
func (d *Dialer) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var errs []error
	if d.session != nil {
		if err := d.session.Close(); err != nil {
			errs = append(errs, err)
		}
		d.session = nil
	}
	if d.conn != nil {
		if err := d.conn.Close(); err != nil {
			errs = append(errs, err)
		}
		d.conn = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing dialer: %v", errs)
	}
	return nil
}

// IsConnected returns true if the dialer has an active yamux session.
// This can be used to check if the dialer is ready to make connections
// without actually attempting a dial operation.
func (d *Dialer) IsConnected() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.session != nil && !d.session.IsClosed()
}
