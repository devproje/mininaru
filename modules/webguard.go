package modules

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

const maxFetchRedirects = 5

var errBlockedAddress error = errors.New("target resolves to a private, local, or reserved address")

var blockedPorts map[string]bool = map[string]bool{
	"22": true, "25": true, "465": true, "587": true,
	"3306": true, "5432": true, "6379": true, "11211": true,
}

var blockedPrefixes []netip.Prefix = fetchBlocked()

var webFetchDialer *net.Dialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 30 * time.Second,
	Control:   fetchControl,
}

var webFetchClient *http.Client = &http.Client{
	Timeout:       30 * time.Second,
	Transport:     fetchTransport(),
	CheckRedirect: fetchRedirect,
}

func fetchBlocked() []netip.Prefix {
	var raw []string
	var entry string
	var current netip.Prefix
	var prefixes []netip.Prefix

	var err error

	raw = []string{
		"100.64.0.0/10",
		"192.0.0.0/24",
		"198.18.0.0/15",
		"192.0.2.0/24",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"240.0.0.0/4",
		"255.255.255.255/32",
		"2002::/16",
		"2001::/32",
		"64:ff9b::/96",
		"64:ff9b:1::/48",
		"100::/64",
		"fec0::/10",
		"2001:db8::/32",
	}

	for _, entry = range raw {
		current, err = netip.ParsePrefix(entry)
		if err != nil {
			continue
		}

		prefixes = append(prefixes, current)
	}

	return prefixes
}

func fetchAddrAllowed(addr netip.Addr) error {
	var prefix netip.Prefix

	if !addr.IsValid() {
		return errBlockedAddress
	}

	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	if addr.IsLoopback() || addr.IsUnspecified() || addr.IsPrivate() {
		return errBlockedAddress
	}
	if addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return errBlockedAddress
	}
	if addr.IsInterfaceLocalMulticast() || addr.IsMulticast() {
		return errBlockedAddress
	}

	for _, prefix = range blockedPrefixes {
		if prefix.Contains(addr) {
			return errBlockedAddress
		}
	}

	return nil
}

func fetchControl(network, address string, c syscall.RawConn) error {
	var parsed netip.AddrPort

	var err error

	parsed, err = netip.ParseAddrPort(address)
	if err != nil {
		return errBlockedAddress
	}

	return fetchAddrAllowed(parsed.Addr())
}

func fetchTarget(raw string) (*url.URL, error) {
	var parsed *url.URL
	var port string
	var addr netip.Addr

	var err error

	if httpURL(raw) == "" {
		return nil, fmt.Errorf("only http and https urls can be fetched")
	}

	parsed, err = url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("url has no host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("url must not carry credentials")
	}

	port = parsed.Port()
	if blockedPorts[port] {
		return nil, fmt.Errorf("port %s is not fetchable", port)
	}

	addr, err = netip.ParseAddr(parsed.Hostname())
	if err == nil {
		err = fetchAddrAllowed(addr)
		if err != nil {
			return nil, err
		}
	}

	return parsed, nil
}

func fetchRedirect(req *http.Request, via []*http.Request) error {
	var err error

	if len(via) >= maxFetchRedirects {
		return fmt.Errorf("stopped after %d redirects", maxFetchRedirects)
	}

	_, err = fetchTarget(req.URL.String())
	if err != nil {
		return err
	}

	return nil
}

func fetchTransport() *http.Transport {
	var transport *http.Transport

	transport = http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = webFetchDialer.DialContext

	return transport
}
