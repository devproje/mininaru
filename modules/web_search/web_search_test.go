// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package web_search

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
)

func TestFormatResultsEmpty(t *testing.T) {
	var got string

	got = formatResults(nil)
	if got != "no results" {
		t.Fatalf("formatResults(nil) = %q, want %q", got, "no results")
	}
}

func TestFormatResultsIncludesEachHit(t *testing.T) {
	var results []searchResult
	var got string

	results = []searchResult{
		{Title: "first", Url: "https://one.example", Snippet: "about one"},
		{Title: "second", Url: "https://two.example"},
	}

	got = formatResults(results)
	if !strings.Contains(got, "first") || !strings.Contains(got, "https://one.example") || !strings.Contains(got, "about one") {
		t.Fatalf("formatResults missing first hit's fields: %q", got)
	}
	if !strings.Contains(got, "second") || !strings.Contains(got, "https://two.example") {
		t.Fatalf("formatResults missing second hit's fields: %q", got)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	var lookup modules.WebBackendLookup
	var err error

	lookup = func() (*modules.WebBackend, error) {
		return &modules.WebBackend{Kind: modules.WebBackendBrave}, nil
	}

	_, err = Search(lookup).Execute(context.Background(), `{"query":""}`)
	if err == nil {
		t.Fatal("expected an error for an empty query")
	}
}

func TestSearchSurfacesLookupError(t *testing.T) {
	var wantErr error
	var lookup modules.WebBackendLookup
	var err error

	wantErr = fmt.Errorf("no web provider is selected")

	lookup = func() (*modules.WebBackend, error) {
		return nil, wantErr
	}

	_, err = Search(lookup).Execute(context.Background(), `{"query":"golang"}`)
	if err == nil {
		t.Fatal("expected the lookup error to surface")
	}
	if err.Error() != wantErr.Error() {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestSearchRejectsUnsupportedBackendKind(t *testing.T) {
	var lookup modules.WebBackendLookup
	var err error

	lookup = func() (*modules.WebBackend, error) {
		return &modules.WebBackend{Kind: "unknown"}, nil
	}

	_, err = Search(lookup).Execute(context.Background(), `{"query":"golang"}`)
	if err == nil {
		t.Fatal("expected an error for an unsupported backend kind")
	}
}
