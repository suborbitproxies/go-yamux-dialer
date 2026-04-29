package yamuxdialer

import (
	"context"
	"testing"
)

func TestWithProxyBasicAuth_RoundTrip(t *testing.T) {
	ctx := WithProxyBasicAuth(context.Background(), "alice", "secret")
	user, pass, ok := proxyAuthFromContext(ctx)
	if !ok {
		t.Fatal("expected credentials from context")
	}
	if user != "alice" || pass != "secret" {
		t.Fatalf("got (%q,%q)", user, pass)
	}
}

func TestProxyAuthFromContext_Absent(t *testing.T) {
	_, _, ok := proxyAuthFromContext(context.Background())
	if ok {
		t.Fatal("expected no credentials without WithProxyBasicAuth")
	}
}
