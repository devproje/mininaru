// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package web_fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/modules/browser"
)

type tavilyExtractResponse struct {
	Results []tavilyExtractResult `json:"results"`
}

type tavilyExtractResult struct {
	Url        string `json:"url"`
	RawContent string `json:"raw_content"`
}

const requestTimeout = 15 * time.Second
const maxResponseBytes = 2 << 20

const tavilyExtractUrl = "https://api.tavily.com/extract"

func isBogon(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

func guardedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var host string
	var port string
	var ips []net.IP
	var ip net.IP
	var dialer net.Dialer

	var err error

	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err = net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %q", host)
	}

	for _, ip = range ips {
		if isBogon(ip) {
			return nil, fmt.Errorf("refusing to fetch %q: resolves to a private or link-local address", host)
		}
	}

	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

func guardedClient() *http.Client {
	var transport http.Transport

	transport = http.Transport{DialContext: guardedDialContext}

	return &http.Client{Transport: &transport, Timeout: requestTimeout}
}

func directFetch(ctx context.Context, target string) (string, error) {
	var parsed *url.URL
	var request *http.Request
	var client *http.Client
	var response *http.Response
	var body []byte
	var contentType string
	var text string

	var err error

	parsed, err = url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https")
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}

	client = guardedClient()

	response, err = client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch failed: %s", response.Status)
	}

	contentType = response.Header.Get("Content-Type")
	if strings.Contains(contentType, "html") {
		text, err = browser.HTMLToText(string(body))
		if err != nil {
			return "", err
		}

		return text, nil
	}

	return string(body), nil
}

func tavilyFetch(ctx context.Context, backend *modules.WebBackend, target string) (string, error) {
	var endpoint string
	var requestBody []byte
	var request *http.Request
	var client http.Client
	var response *http.Response
	var body []byte
	var payload tavilyExtractResponse

	var err error

	endpoint = backend.BaseUrl
	if endpoint == "" {
		endpoint = tavilyExtractUrl
	}

	requestBody, err = json.Marshal(map[string]any{
		"api_key": backend.ApiKey,
		"urls":    []string{target},
	})
	if err != nil {
		return "", err
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	client = http.Client{Timeout: requestTimeout}

	response, err = client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tavily extract failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	err = json.Unmarshal(body, &payload)
	if err != nil {
		return "", err
	}

	if len(payload.Results) == 0 {
		return "", fmt.Errorf("tavily extract returned no content for %q", target)
	}

	return payload.Results[0].RawContent, nil
}

func Fetch(lookup modules.WebBackendLookup) modules.Tool {
	return modules.Tool{
		Name:        "web_fetch",
		Description: "Fetch a single URL and return its content as plain text. Refuses to fetch private, loopback, and link-local addresses.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string"},
			},
			"required":             []string{"url"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Url string `json:"url"`
			}
			var backend *modules.WebBackend

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Url == "" {
				return "", fmt.Errorf("url is required")
			}

			backend, err = lookup()
			if err == nil && backend.Kind == modules.WebBackendTavily {
				return tavilyFetch(ctx, backend, payload.Url)
			}

			return directFetch(ctx, payload.Url)
		},
	}
}

func Tools(lookup modules.WebBackendLookup) []modules.Tool {
	return []modules.Tool{Fetch(lookup)}
}
