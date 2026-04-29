# go-yamux-dialer

A configurable Golang yamux dialer that works as a drop-in replacement for regular dialers, allowing you to make HTTP/HTTPS requests through a yamux proxy server using the standard HTTP client.

## Features

- **Drop-in replacement**: Works seamlessly with standard `net.Dialer` and `http.Client`
- **Configurable**: Extensive configuration options for timeouts, keep-alive, compression, etc.
- **Connection reuse**: Maintains a single yamux session and multiplexes multiple streams
- **Thread-safe**: Safe for concurrent use
- **Automatic reconnection**: Handles session failures and automatically reconnects
- **Per-stream proxy auth**: Use different Basic credentials per HTTP request (or dial) while sharing one yamux session

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

Authentication is carried on **each** HTTP `CONNECT` on a **new yamux stream**. The TCP connection and yamux session stay shared; only the `Proxy-Authorization` header changes when you vary credentials.

### Per-stream (per-request) proxy authentication

`Config.ProxyUsername` / `Config.ProxyPassword` apply to every dial that does not supply credentials on the context. To use **different** username and password for specific requests while keeping a **single** `Dialer` (one yamux session), attach credentials to the request context with `WithProxyBasicAuth`. The standard library’s `http.Transport` passes the request context into `DialContext`, so each `http.Request` can carry its own proxy Basic auth.

```go
import (
    "context"
    "net/http"
)

dialer, _ := yamuxdialer.New(yamuxdialer.DefaultConfig("proxy.example.com:443"))
defer dialer.Close()

client := &http.Client{
    Transport: &http.Transport{
        DialContext:       dialer.DialContext,
        DisableKeepAlives: true, // recommended when each request should map to a fresh stream (e.g. rotating identity)
    },
}

ctx := yamuxdialer.WithProxyBasicAuth(context.Background(), "user-session-a", "mypassword")
req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/", nil)
if err != nil {
    panic(err)
}

resp, err := client.Do(req)
if err != nil {
    panic(err)
}
defer resp.Body.Close()
```

Context values override the static username and password from `Config` for that dial only. If neither context nor config provides credentials, the `CONNECT` request is sent without `Proxy-Authorization`.

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
- **MaxStreamWindowSize** (uint32): Yamux maximum stream window; `DefaultConfig` uses 1 MiB. Zero leaves the upstream yamux default; if set explicitly, must be at least yamux’s initial window (256 KiB).
- **LogOutput** (io.Writer): Optional logger implementing `io.Writer` interface
- **ProxyUsername** (string, optional): Default username for proxy Basic auth on `CONNECT` when the context passed to `DialContext` does not include credentials from `WithProxyBasicAuth`
- **ProxyPassword** (string, optional): Default password for proxy Basic auth, same precedence as username

## Protocol

The dialer sends an HTTP CONNECT request to the proxy server in the format:
```
CONNECT <target-address> HTTP/1.1\r\n
Host: <target-address>\r\n
Proxy-Authorization: Basic <base64-encoded-credentials>\r\n
\r\n
```

If authentication credentials are provided (either via `WithProxyBasicAuth` on the context passed to `DialContext`, or `Config.ProxyUsername` / `Config.ProxyPassword` when the context carries no embedded credentials), they are sent as a Basic Authorization header using base64 encoding. Each new stream issues its own `CONNECT`, so credentials can differ per stream without opening a new yamux session. The proxy should validate `CONNECT`, establish a connection to the target address, and forward data bidirectionally.

## Thread Safety

The dialer is safe for concurrent use. Multiple goroutines can call `Dial` or `DialContext` simultaneously, and they will share the same underlying yamux session.

## License

MIT

