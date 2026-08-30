// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/devproje/mininaru/modules/client"
)

func clientExecute() error {
	var gateways []client.Gateway

	var err error

	gateways, err = gatewayList()
	if err != nil {
		return err
	}

	return client.Run(client.Options{
		Url:      promptUrlRef,
		Session:  promptSessionRef,
		Agent:    promptAgentRef,
		ApiKey:   promptApiKeyRef,
		Gateways: gateways,
	})
}
