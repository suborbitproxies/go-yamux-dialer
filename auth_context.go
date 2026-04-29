package yamuxdialer

import "context"

type proxyBasicAuthCtxKey struct{}

type proxyBasicAuth struct {
	username string
	password string
}

// WithProxyBasicAuth attaches HTTP Basic proxy credentials to ctx for use with
// DialContext. When present, these override Config.ProxyUsername and
// Config.ProxyPassword for that dial only (each yamux stream sends its own CONNECT).
//
// http.Transport passes the request context into DialContext, so use this with
// http.NewRequestWithContext to vary credentials per HTTP request while sharing
// a single Dialer and yamux session.
func WithProxyBasicAuth(ctx context.Context, username, password string) context.Context {
	return context.WithValue(ctx, proxyBasicAuthCtxKey{}, proxyBasicAuth{
		username: username,
		password: password,
	})
}

func proxyAuthFromContext(ctx context.Context) (username, password string, ok bool) {
	v := ctx.Value(proxyBasicAuthCtxKey{})
	if v == nil {
		return "", "", false
	}
	a, ok := v.(proxyBasicAuth)
	if !ok {
		return "", "", false
	}
	return a.username, a.password, true
}
