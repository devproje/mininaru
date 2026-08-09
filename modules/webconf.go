// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/devproje/mininaru/util"
)

type SearchConfig struct {
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
}

type WebConfig struct {
	Search SearchConfig `json:"search"`
}

const WEB_PATH = "web.json"

const (
	ProviderDuckDuckGo = "duckduckgo"
	ProviderSearXNG    = "searxng"
	ProviderBrave      = "brave"
	ProviderTavily     = "tavily"
)

var web WebConfig

var webMu sync.RWMutex

var defaultWeb WebConfig = WebConfig{Search: SearchConfig{Provider: ProviderDuckDuckGo}}

func WebSearchConfig() SearchConfig {
	webMu.RLock()
	defer webMu.RUnlock()

	return web.Search
}

func WebSetSearch(cfg SearchConfig) {
	webMu.Lock()
	defer webMu.Unlock()

	web.Search = cfg
}

func WebProviders() []string {
	return []string{ProviderDuckDuckGo, ProviderSearXNG, ProviderBrave, ProviderTavily}
}

func WebValidate(cfg *SearchConfig) error {
	switch cfg.Provider {
	case ProviderDuckDuckGo:
		return nil
	case ProviderSearXNG:
		if !strings.HasPrefix(cfg.Endpoint, "http://") && !strings.HasPrefix(cfg.Endpoint, "https://") {
			return fmt.Errorf("searxng needs an http or https endpoint")
		}

		return nil
	case ProviderBrave, ProviderTavily:
		if cfg.APIKey == "" {
			return fmt.Errorf("%s needs an api key", cfg.Provider)
		}

		return nil
	}

	return fmt.Errorf("unknown search provider %q, expected one of %s",
		cfg.Provider, strings.Join(WebProviders(), ", "))
}

func webAccept(loaded WebConfig) WebConfig {
	var err error

	err = WebValidate(&loaded.Search)
	if err != nil {
		util.Log.Warn("ignoring an invalid search config",
			"config", WEB_PATH, "error", err, "fallback", defaultWeb.Search.Provider)

		return defaultWeb
	}

	return loaded
}

func WebSave() error {
	var path string
	var buf []byte

	var err error

	webMu.RLock()
	buf, err = json.MarshalIndent(web, "", "    ")
	webMu.RUnlock()
	if err != nil {
		return err
	}

	path = util.Path(WEB_PATH)

	return util.WriteFileAtomic(path, buf, 0600)
}

func WebLoad() error {
	var path string
	var buf []byte
	var loaded WebConfig

	var err error

	webMu.Lock()
	web = defaultWeb
	webMu.Unlock()

	path = util.Path(WEB_PATH)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(defaultWeb, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	loaded = defaultWeb

	err = json.Unmarshal(buf, &loaded)
	if err != nil {
		return err
	}

	loaded.Search.Provider = strings.ToLower(strings.TrimSpace(loaded.Search.Provider))
	if loaded.Search.Provider == "" {
		loaded.Search.Provider = defaultWeb.Search.Provider
	}

	loaded = webAccept(loaded)

	webMu.Lock()
	web = loaded
	webMu.Unlock()

	return nil
}

func WebReload() error {
	return WebLoad()
}
