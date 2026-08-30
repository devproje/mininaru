// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"net/http"

	"github.com/devproje/mininaru/modules/client"
	"github.com/spf13/cobra"
)

type remoteRef struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func remoteTarget(cmd *cobra.Command) (string, string, bool, error) {
	var gw *client.Gateway
	var rawURL string
	var apiKey string
	var base string

	var err error

	gw, err = resolveGateway()
	if err != nil {
		return "", "", false, err
	}

	if gw != nil {
		if cmd.Flags().Changed("url") {
			return "", "", false, fmt.Errorf("--gateway cannot be combined with --url")
		}

		rawURL = gw.Url
		apiKey = gw.ApiKey
	} else if cmd.Flags().Changed("url") {
		rawURL = promptUrlRef
		apiKey = promptApiKeyRef
	} else {
		return "", "", false, nil
	}

	base, err = client.ApiBase(rawURL)
	if err != nil {
		return "", "", false, err
	}

	return base, client.ResolveApiKey(apiKey, rawURL), true, nil
}

func remoteEnabled(cmd *cobra.Command) (bool, error) {
	var remote bool

	var err error

	_, _, remote, err = remoteTarget(cmd)

	return remote, err
}

func remoteGet(cmd *cobra.Command, path string, out any) (bool, error) {
	var base string
	var apiKey string
	var remote bool

	var err error

	base, apiKey, remote, err = remoteTarget(cmd)
	if err != nil || !remote {
		return remote, err
	}

	return true, client.Api(http.MethodGet, base+path, apiKey, nil, out)
}

func remoteResolveId(cmd *cobra.Command, listPath string, ref string) (string, bool, error) {
	var items []remoteRef
	var item remoteRef
	var remote bool

	var err error

	remote, err = remoteGet(cmd, listPath, &items)
	if err != nil || !remote {
		return "", remote, err
	}

	for _, item = range items {
		if item.Id == ref || item.Name == ref {
			return item.Id, true, nil
		}
	}

	return "", true, fmt.Errorf("%q not found on the remote", ref)
}

func remoteDo(cmd *cobra.Command, method string, path string) (bool, error) {
	var base string
	var apiKey string
	var remote bool

	var err error

	base, apiKey, remote, err = remoteTarget(cmd)
	if err != nil || !remote {
		return remote, err
	}

	return true, client.Api(method, base+path, apiKey, nil, nil)
}
