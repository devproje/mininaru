// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/client"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

func remoteTestCmd() *cobra.Command {
	var cmd cobra.Command

	cmd.Flags().StringVar(&promptUrlRef, "url", client.DefaultUrl, "")
	cmd.Flags().StringVar(&promptApiKeyRef, "api-key", "", "")
	cmd.Flags().StringVar(&gatewayRef, "gateway", "", "")

	return &cmd
}

func TestRemoteTargetPrecedence(t *testing.T) {
	var cmd *cobra.Command
	var remote bool
	var base string

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	defer func() { gatewayRef = "" }()

	cmd = remoteTestCmd()
	_, _, remote, err = remoteTarget(cmd)
	if err != nil || remote {
		t.Fatalf("no flags should stay local, got remote=%v err=%v", remote, err)
	}

	cmd = remoteTestCmd()
	err = cmd.Flags().Set("url", "ws://box:8223/ws")
	if err != nil {
		t.Fatal(err)
	}

	base, _, remote, err = remoteTarget(cmd)
	if err != nil || !remote || base != "http://box:8223/api" {
		t.Fatalf("--url should go remote: base=%q remote=%v err=%v", base, remote, err)
	}

	err = saveGateways(gatewayStore{"prod": {Url: "ws://gw:9000/ws", ApiKey: "sk-9"}})
	if err != nil {
		t.Fatal(err)
	}

	cmd = remoteTestCmd()
	gatewayRef = "prod"
	base, _, remote, err = remoteTarget(cmd)
	if err != nil || !remote || base != "http://gw:9000/api" {
		t.Fatalf("--gateway should go remote: base=%q remote=%v err=%v", base, remote, err)
	}

	cmd = remoteTestCmd()
	gatewayRef = "prod"
	err = cmd.Flags().Set("url", "ws://box/ws")
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = remoteTarget(cmd)
	if err == nil {
		t.Fatal("--gateway with --url should error")
	}
}

func TestRemoteGetHitsTheApi(t *testing.T) {
	var srv *httptest.Server
	var cmd *cobra.Command
	var list []*core.Provider
	var gotPath string
	var gotAuth string

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	defer func() { gatewayRef = "" }()

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		json.NewEncoder(w).Encode([]*core.Provider{{Id: "p1", Name: "openai"}})
	}))
	defer srv.Close()

	err = saveGateways(gatewayStore{"t": {Url: srv.URL, ApiKey: "sk-t"}})
	if err != nil {
		t.Fatal(err)
	}

	cmd = remoteTestCmd()
	gatewayRef = "t"

	_, err = remoteGet(cmd, "/providers", &list)
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/providers" {
		t.Fatalf("path = %q", gotPath)
	}

	if gotAuth != "Bearer sk-t" {
		t.Fatalf("auth = %q", gotAuth)
	}

	if len(list) != 1 || list[0].Name != "openai" {
		t.Fatalf("decoded = %+v", list)
	}
}
