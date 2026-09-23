// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package web_fetch

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsBogonRejectsPrivateAndLinkLocalRanges(t *testing.T) {
	var blocked []string
	var raw string
	var ip net.IP

	blocked = []string{
		"127.0.0.1",
		"::1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"fe80::1",
		"0.0.0.0",
	}

	for _, raw = range blocked {
		ip = net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("failed to parse test address %q", raw)
		}
		if !isBogon(ip) {
			t.Fatalf("expected %q to be treated as a bogon address", raw)
		}
	}
}

func TestIsBogonAllowsPublicAddresses(t *testing.T) {
	var allowed []string
	var raw string
	var ip net.IP

	allowed = []string{"8.8.8.8", "1.1.1.1"}

	for _, raw = range allowed {
		ip = net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("failed to parse test address %q", raw)
		}
		if isBogon(ip) {
			t.Fatalf("expected %q to be treated as a public address", raw)
		}
	}
}

func TestGuardedDialContextRefusesLoopback(t *testing.T) {
	var conn net.Conn

	var err error

	conn, err = guardedDialContext(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		conn.Close()
		t.Fatal("expected the guard to refuse a loopback address")
	}
	if !strings.Contains(err.Error(), "private or link-local") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDirectFetchRejectsNonHttpScheme(t *testing.T) {
	var err error

	_, err = directFetch(context.Background(), "file:///etc/passwd")
	if err == nil {
		t.Fatal("expected an error for a non-http(s) scheme")
	}
}

func TestDirectFetchRefusesLoopbackTarget(t *testing.T) {
	var server *httptest.Server

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should never be reached"))
	}))
	t.Cleanup(server.Close)

	_, err = directFetch(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected the SSRF guard to refuse a loopback target")
	}
}
