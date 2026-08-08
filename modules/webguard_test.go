package modules

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestFetchAddrAllowed(t *testing.T) {
	var blocked []string
	var allowed []string
	var entry string
	var addr netip.Addr

	var err error

	blocked = []string{
		"127.0.0.1", "::1",
		"10.0.0.1", "172.16.0.1", "192.168.1.1", "fd00::1",
		"169.254.169.254", "fe80::1",
		"100.64.0.1", "100.100.100.100",
		"0.0.0.0", "::",
		"ff02::1", "224.0.0.1",
		"2002:0a00:0001::", "64:ff9b::a00:1",
		"255.255.255.255", "240.0.0.1",
		"198.18.0.1", "192.0.2.1", "2001:db8::1",
	}

	allowed = []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "2606:4700::1111"}

	for _, entry = range blocked {
		addr, err = netip.ParseAddr(entry)
		if err != nil {
			t.Fatal(err)
		}
		if fetchAddrAllowed(addr) == nil {
			t.Fatalf("%s was allowed", entry)
		}
	}

	for _, entry = range allowed {
		addr, err = netip.ParseAddr(entry)
		if err != nil {
			t.Fatal(err)
		}
		if fetchAddrAllowed(addr) != nil {
			t.Fatalf("%s was blocked", entry)
		}
	}
}

func TestFetchAddrAllowedUnmapsIPv4In6(t *testing.T) {
	var entry string
	var addr netip.Addr

	var err error

	for _, entry = range []string{"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254"} {
		addr, err = netip.ParseAddr(entry)
		if err != nil {
			t.Fatal(err)
		}
		if fetchAddrAllowed(addr) == nil {
			t.Fatalf("%s was allowed, the ipv4-in-ipv6 unmap must run before the predicates", entry)
		}
	}
}

func TestWebFetchRefusesLoopback(t *testing.T) {
	var server *httptest.Server
	var reached bool

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	defer server.Close()

	_, err = WebFetch().Execute(context.Background(), `{"url":"`+server.URL+`"}`)
	if err == nil {
		t.Fatal("web_fetch reached a loopback server")
	}
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("error = %v, want errBlockedAddress", err)
	}
	if reached {
		t.Fatal("the handler ran, so the connection was actually made")
	}
}

func TestWebFetchRejectsScheme(t *testing.T) {
	var entry string

	var err error

	for _, entry = range []string{"file:///etc/passwd", "gopher://example.com/", "ftp://example.com/x"} {
		_, err = WebFetch().Execute(context.Background(), `{"url":"`+entry+`"}`)
		if err == nil {
			t.Fatalf("%s was fetched", entry)
		}
		if !strings.Contains(err.Error(), "http and https") {
			t.Fatalf("%s error = %v", entry, err)
		}
	}
}

func TestWebFetchRejectsCredentialsAndPorts(t *testing.T) {
	var err error

	_, err = WebFetch().Execute(context.Background(), `{"url":"http://user:pass@example.com/"}`)
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("credential url error = %v", err)
	}

	_, err = WebFetch().Execute(context.Background(), `{"url":"http://example.com:6379/"}`)
	if err == nil || !strings.Contains(err.Error(), "not fetchable") {
		t.Fatalf("blocked port error = %v", err)
	}
}
