// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package modules

type WebBackend struct {
	Kind    string
	ApiKey  string
	BaseUrl string
}

type WebBackendLookup func() (*WebBackend, error)

const (
	WebBackendBrave  = "brave"
	WebBackendTavily = "tavily"
	WebBackendOllama = "ollama"
)
