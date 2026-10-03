// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/devproje/mininaru/modules/client"
	"github.com/devproje/mininaru/modules/tui"
)

func clientExecute() error {
	var gateways []client.Gateway

	var err error

	gateways, err = gatewayList()
	if err != nil {
		return err
	}

	return tui.RunTuiSession(client.Options{
		Url:      promptUrlRef,
		Session:  promptSessionRef,
		Agent:    promptAgentRef,
		ApiKey:   promptApiKeyRef,
		Cwd:      promptCwdRef,
		NoCache:  promptNoCacheRef,
		Resume:   promptResumeRef,
		Gateways: gateways,
	})
}
