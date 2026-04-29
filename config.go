package yamuxdialer

import (
	"io"
	"time"
)

// Config holds configuration options for the yamux dialer.
// Use DefaultConfig to create a config with sensible defaults, then
// customize as needed.
type Config struct {
	// ProxyAddress is the address of the yamux proxy server (e.g., "proxy.example.com:443")
	ProxyAddress string

	// DialTimeout is the timeout for establishing the initial connection to the proxy
	DialTimeout time.Duration

	// StreamOpenTimeout is the timeout for opening a new stream on the yamux session
	StreamOpenTimeout time.Duration

	// KeepAliveInterval is the interval at which keep-alive messages are sent
	// Set to 0 to disable keep-alive
	KeepAliveInterval time.Duration

	// ConnectionWriteTimeout is the timeout for writing to the connection
	ConnectionWriteTimeout time.Duration

	// MaxStreamWindowSize is the maximum yamux stream window (bytes).
	// Zero leaves the yamux default; if set, must be at least 256 KiB (yamux requirement).
	MaxStreamWindowSize uint32

	// LogOutput can be set to enable logging (optional)
	// Must implement io.Writer interface
	LogOutput io.Writer

	// ProxyUsername is the default username for proxy authentication (optional).
	// Override per dial with WithProxyBasicAuth on the context passed to DialContext.
	ProxyUsername string

	// ProxyPassword is the default password for proxy authentication (optional).
	// Override per dial with WithProxyBasicAuth on the context passed to DialContext.
	ProxyPassword string
}

// DefaultConfig returns a config with sensible defaults for the given proxy address.
//
// The defaults are:
//   - DialTimeout: 30 seconds
//   - StreamOpenTimeout: 10 seconds
//   - KeepAliveInterval: 30 seconds
//   - ConnectionWriteTimeout: 10 seconds
//   - MaxStreamWindowSize: 1MB
//
// Example:
//
//	config := yamuxdialer.DefaultConfig("proxy.example.com:443")
//	config.ProxyUsername = "user"
//	config.ProxyPassword = "pass"
//	dialer, _ := yamuxdialer.New(config)
func DefaultConfig(proxyAddress string) *Config {
	return &Config{
		ProxyAddress:           proxyAddress,
		DialTimeout:            30 * time.Second,
		StreamOpenTimeout:      10 * time.Second,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    1024 * 1024, // 1MB
	}
}
