// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

type Provider struct {
	Id               string `json:"id"`
	Name             string `json:"name"`
	ApiKey           string `json:"api_key"`
	BaseURL          string `json:"base_url"`
	Kind             string `json:"kind,omitempty"`
	Cache            string `json:"cache,omitempty"`
	ResponseCache    bool   `json:"response_cache,omitempty"`
	ResponseCacheTTL int    `json:"response_cache_ttl,omitempty"`
}

type ProviderConfig struct {
	DefaultId string      `json:"default_id"`
	Providers []*Provider `json:"providers"`
}

const (
	PROVIDER_PATH = "provider.json"

	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	CacheAuto        = "auto"
	CacheOff         = "off"
	CacheEphemeral   = "ephemeral"
	CacheEphemeral1h = "ephemeral_1h"
)

var Providers []*Provider
var DefaultProvider *Provider

var emptyProviderObj ProviderConfig = ProviderConfig{Providers: []*Provider{}}

func (p *Provider) ProviderKind() string {
	if p != nil && p.Kind == ProviderAnthropic {
		return ProviderAnthropic
	}

	return ProviderOpenAI
}

func (p *Provider) CachePolicy() string {
	if p == nil || p.Cache == "" {
		return CacheAuto
	}

	return p.Cache
}

func isOpenRouter(baseURL string) bool {
	var parsed *url.URL
	var host string

	var err error

	parsed, err = url.Parse(baseURL)
	if err != nil {
		return false
	}
	host = strings.ToLower(parsed.Hostname())

	return host == "openrouter.ai" || strings.HasSuffix(host, ".openrouter.ai")
}

func applyOpenAICache(params *openai.ChatCompletionNewParams, provider *Provider) {
	var control map[string]any
	var policy string

	if params == nil || provider == nil {
		return
	}
	policy = provider.CachePolicy()
	if policy == CacheAuto && isOpenRouter(provider.BaseURL) && strings.HasPrefix(params.Model, "anthropic/") {
		policy = CacheEphemeral
	}
	if policy != CacheEphemeral && policy != CacheEphemeral1h {
		return
	}

	control = map[string]any{"type": "ephemeral"}
	if policy == CacheEphemeral1h {
		control["ttl"] = "1h"
	}
	params.SetExtraFields(map[string]any{"cache_control": control})
}

func ProviderValidate(provider Provider) error {
	var kind string
	var cache string

	kind = provider.Kind
	if kind == "" {
		kind = ProviderOpenAI
	}
	cache = provider.CachePolicy()
	if kind != ProviderOpenAI && kind != ProviderAnthropic {
		return fmt.Errorf("unsupported provider kind %q", provider.Kind)
	}
	if cache != CacheAuto && cache != CacheOff && cache != CacheEphemeral && cache != CacheEphemeral1h {
		return fmt.Errorf("unsupported cache policy %q", provider.Cache)
	}
	if provider.ResponseCache && !isOpenRouter(provider.BaseURL) {
		return fmt.Errorf("response caching is only supported for OpenRouter providers")
	}
	if provider.ResponseCacheTTL < 0 || provider.ResponseCacheTTL > 86400 {
		return fmt.Errorf("response cache TTL must be 0 or between 1 and 86400 seconds")
	}

	return nil
}

func ProviderFind(ref string) (*Provider, error) {
	var cur *Provider

	var err error

	for _, cur = range Providers {
		if cur.Id != ref {
			continue
		}

		return cur, nil
	}

	for _, cur = range Providers {
		if cur.Name != ref {
			continue
		}

		return cur, nil
	}

	err = fmt.Errorf("provider %s not found", ref)

	return nil, err
}

func ProviderInit() error {
	var path string
	var buf []byte
	var cfg ProviderConfig
	var provider *Provider

	var err error

	path = util.Path(PROVIDER_PATH)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(emptyProviderObj, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(buf, &cfg)
	if err != nil {
		err = json.Unmarshal(buf, &cfg.Providers)
		if err != nil {
			return err
		}
	}

	Providers = cfg.Providers
	for _, provider = range Providers {
		err = ProviderValidate(*provider)
		if err != nil {
			return fmt.Errorf("provider %s: %w", provider.Name, err)
		}
	}
	DefaultProvider = nil

	if cfg.DefaultId != "" {
		DefaultProvider, _ = ProviderFind(cfg.DefaultId)
	}

	if DefaultProvider == nil && len(Providers) > 0 {
		DefaultProvider = Providers[0]
	}

	return nil
}

func ProviderSave() error {
	var cfg ProviderConfig
	var path string
	var buf []byte

	var err error

	cfg = ProviderConfig{Providers: Providers}
	if DefaultProvider != nil {
		cfg.DefaultId = DefaultProvider.Id
	}

	path = util.Path(PROVIDER_PATH)
	buf, err = json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(path, buf, 0600)
	if err != nil {
		return err
	}

	return nil
}

func ProviderCreate(payload Provider) {
	payload.Id = uuid.NewString()
	Providers = append(Providers, &payload)

	if DefaultProvider == nil {
		DefaultProvider = &payload
	}
}

func ProviderDefault(ref string) error {
	var prov *Provider

	var err error

	prov, err = ProviderFind(ref)
	if err != nil {
		return err
	}

	DefaultProvider = prov

	return ProviderSave()
}

func ProviderUpdateConfig(id string, name, apiKey, baseURL, kind, cache *string, responseCache *bool, responseCacheTTL *int) error {
	var index int
	var current *Provider
	var update Provider

	var err error

	for index, current = range Providers {
		if current.Id != id && current.Name != id {
			continue
		}
		update = *current
		if name != nil {
			update.Name = *name
		}
		if apiKey != nil {
			update.ApiKey = *apiKey
		}
		if baseURL != nil {
			update.BaseURL = *baseURL
		}
		if kind != nil {
			update.Kind = *kind
		}
		if cache != nil {
			update.Cache = *cache
		}
		if responseCache != nil {
			update.ResponseCache = *responseCache
		}
		if responseCacheTTL != nil {
			update.ResponseCacheTTL = *responseCacheTTL
		}
		err = ProviderValidate(update)
		if err != nil {
			return err
		}
		if DefaultProvider == current {
			DefaultProvider = &update
		}
		Providers[index] = &update
		return ProviderSave()
	}

	return fmt.Errorf("cannot find provider id for %s", id)
}

func ProviderUpdateOptions(id string, kind, cache *string, responseCache *bool, responseCacheTTL *int) error {
	return ProviderUpdateConfig(id, nil, nil, nil, kind, cache, responseCache, responseCacheTTL)
}

func ProviderUpdateFields(id string, name, apiKey, baseURL *string) error {
	return ProviderUpdateConfig(id, name, apiKey, baseURL, nil, nil, nil, nil)
}

func ProviderUpdate(id string, payload Provider) error {
	var name, apiKey, baseURL *string

	if payload.Name != "" {
		name = &payload.Name
	}
	if payload.ApiKey != "" {
		apiKey = &payload.ApiKey
	}
	if payload.BaseURL != "" {
		baseURL = &payload.BaseURL
	}

	return ProviderUpdateFields(id, name, apiKey, baseURL)
}

func ProviderDelete(id string) error {
	var index int
	var cur *Provider
	var agent *NaruAgent

	var err error

	for index, cur = range Providers {
		if cur.Id != id {
			continue
		}

		if Global != nil && Global.ProviderId == id {
			return fmt.Errorf("provider is used by global agent %s", Global.Id)
		}

		for _, agent = range Agents {
			if agent.ProviderId == id {
				return fmt.Errorf("provider is used by agent %s", agent.Id)
			}
		}

		Providers = append(Providers[:index], Providers[index+1:]...)

		if DefaultProvider == cur {
			DefaultProvider = nil

			if len(Providers) > 0 {
				DefaultProvider = Providers[0]
			}
		}

		err = ProviderSave()
		if err != nil {
			return err
		}

		return nil
	}

	err = fmt.Errorf("provider is not found, aborted.")
	return err
}
