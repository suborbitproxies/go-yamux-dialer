package yamuxdialer_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	yamuxdialer "github.com/suborbitproxies/go-yamux-dialer"
)

func ExampleNew() {
	// Create a yamux dialer with default configuration
	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	// Create HTTP client using the yamux dialer
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
		Timeout: 30 * time.Second,
	}

	// Make HTTP request through the yamux proxy
	resp, err := client.Get("https://example.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
}

func ExampleNew_customConfig() {
	// Create a yamux dialer with custom configuration
	config := &yamuxdialer.Config{
		ProxyAddress:           "proxy.example.com:443",
		DialTimeout:            30 * time.Second,
		StreamOpenTimeout:      10 * time.Second,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    512 * 1024, // 512KB
	}

	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	// Use the dialer...
	_ = dialer
}

func ExampleNew_withAuthentication() {
	// Create a yamux dialer with proxy authentication
	config := &yamuxdialer.Config{
		ProxyAddress:  "proxy.example.com:443",
		ProxyUsername: "myuser",
		ProxyPassword: "mypassword",
	}

	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	// Create HTTP client using the authenticated yamux dialer
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
		Timeout: 30 * time.Second,
	}

	// Make HTTP request through authenticated proxy
	resp, err := client.Get("https://example.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
}

func ExampleWithProxyBasicAuth() {
	// One Dialer keeps a single yamux session; different requests can use different
	// proxy Basic auth by attaching credentials to each request context.
	config := &yamuxdialer.Config{
		ProxyAddress: "proxy.example.com:443",
	}
	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:       dialer.DialContext,
			DisableKeepAlives: true,
		},
		Timeout: 30 * time.Second,
	}

	ctx := yamuxdialer.WithProxyBasicAuth(context.Background(), "user-a", "pass-a")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/", nil)
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
}

func ExampleDialer_DialContext() {
	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	// Dial with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", "example.com:443")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Use the connection...
	_ = conn
}

func ExampleDialer_Dial() {
	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	// Simple dial (uses background context)
	conn, err := dialer.Dial("tcp", "example.com:443")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Use the connection...
	_ = conn
}

func ExampleDialer_IsConnected() {
	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
	dialer, err := yamuxdialer.New(config)
	if err != nil {
		panic(err)
	}
	defer dialer.Close()

	// Check if dialer is connected
	if dialer.IsConnected() {
		fmt.Println("Dialer is connected")
	} else {
		fmt.Println("Dialer is not connected")
	}
}
