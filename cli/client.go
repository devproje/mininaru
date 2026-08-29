// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/devproje/mininaru/client"
)

func clientExecute() error {
	return client.Run(client.Options{
		Url:     promptUrlRef,
		Session: promptSessionRef,
		Agent:   promptAgentRef,
		ApiKey:  promptApiKeyRef,
	})
}
