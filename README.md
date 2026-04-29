# go-yamux-dialer

A configurable Golang yamux dialer that works as a drop-in replacement for regular dialers, allowing you to make HTTP/HTTPS requests through a yamux proxy server using the standard HTTP client.

## Features

- **Drop-in replacement**: Works seamlessly with standard `net.Dialer` and `http.Client`
- **Configurable**: Extensive configuration options for timeouts, keep-alive, compression, etc.
- **Connection reuse**: Maintains a single yamux session and multiplexes multiple streams
- **Thread-safe**: Safe for concurrent use
- **Automatic reconnection**: Handles session failures and automatically reconnects

## Installation

```bash
go get github.com/suborbitproxies/go-yamux-dialer
```

## Usage

### Basic Usage

```go
package main

import (
    "net/http"
    "time"
    
    yamuxdialer "github.com/suborbitproxies/go-yamux-dialer"
)

func main() {
    // Create a yamux dialer with default config
    config := yamuxdialer.DefaultConfig("proxy.example.com:443")
    dialer, err := yamuxdialer.New(config)
    if err != nil {
        panic(err)
    }
    defer dialer.Close()

    // Create HTTP client with custom transport
    client := &http.Client{
        Transport: &http.Transport{
            DialContext: dialer.DialContext,
        },
        Timeout: 30 * time.Second,
    }

    // Make HTTP request
    resp, err := client.Get("https://example.com")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    // Use response...
}
```

### Custom Configuration

```go
config := &yamuxdialer.Config{
    ProxyAddress:           "proxy.example.com:443",
    DialTimeout:            30 * time.Second,
    StreamOpenTimeout:      10 * time.Second,
    KeepAliveInterval:      30 * time.Second,
    ConnectionWriteTimeout: 10 * time.Second,
    MaxStreamWindowSize:    256 * 1024, // 256KB
}

dialer, err := yamuxdialer.New(config)
```

### With Proxy Authentication

```go
config := &yamuxdialer.Config{
    ProxyAddress: "proxy.example.com:443",
    ProxyUsername: "myuser",
    ProxyPassword: "mypassword",
}

dialer, err := yamuxdialer.New(config)
if err != nil {
    panic(err)
}
defer dialer.Close()

// Use the dialer with HTTP client
client := &http.Client{
    Transport: &http.Transport{
        DialContext: dialer.DialContext,
    },
}

resp, err := client.Get("https://example.com")
```

### With HTTPS

The dialer works transparently with HTTPS. The HTTP client will handle TLS negotiation after the connection is established:

```go
client := &http.Client{
    Transport: &http.Transport{
        DialContext: dialer.DialContext,
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: false, // Set to true only for testing
        },
    },
}

resp, err := client.Get("https://api.example.com/data")
```

## Configuration Options

- **ProxyAddress** (string, required): Address of the yamux proxy server
- **DialTimeout** (time.Duration): Timeout for establishing connection to proxy (default: 30s)
- **StreamOpenTimeout** (time.Duration): Timeout for opening a new stream (default: 10s)
- **KeepAliveInterval** (time.Duration): Interval for keep-alive messages (default: 30s, 0 to disable)
- **ConnectionWriteTimeout** (time.Duration): Timeout for writing to connection (default: 10s)
- **MaxStreamWindowSize** (uint32): Maximum window size for streams (default: 256KB)
- **LogOutput** (io.Writer): Optional logger implementing `io.Writer` interface
- **ProxyUsername** (string, optional): Username for proxy authentication
- **ProxyPassword** (string, optional): Password for proxy authentication

## Protocol

The dialer sends an HTTP CONNECT request to the proxy server in the format:
```
CONNECT <target-address> HTTP/1.1\r\n
Host: <target-address>\r\n
Proxy-Authorization: Basic <base64-encoded-credentials>\r\n
\r\n
```

If authentication credentials are provided, they are sent as a Basic Authentication header using base64 encoding. The proxy server should handle this request and establish a connection to the target address, then forward data bidirectionally.

## Thread Safety

The dialer is safe for concurrent use. Multiple goroutines can call `Dial` or `DialContext` simultaneously, and they will share the same underlying yamux session.

## License

MIT

