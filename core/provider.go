// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type Provider struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	ApiKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

type ProviderConfig struct {
	DefaultId string      `json:"default_id"`
	Providers []*Provider `json:"providers"`
}

const PROVIDER_PATH = "provider.json"

var Providers []*Provider
var DefaultProvider *Provider

var emptyProviderObj ProviderConfig = ProviderConfig{Providers: []*Provider{}}

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

func ProviderUpdateFields(id string, name, apiKey, baseURL *string) error {
	var index int
	var cur *Provider
	var update Provider

	var err error

	for index, cur = range Providers {
		if cur.Id != id {
			continue
		}

		update = *cur

		if name != nil {
			update.Name = *name
		}

		if apiKey != nil {
			update.ApiKey = *apiKey
		}

		if baseURL != nil {
			update.BaseURL = *baseURL
		}

		if DefaultProvider == cur {
			DefaultProvider = &update
		}

		Providers[index] = &update
		err = ProviderSave()
		if err != nil {
			return err
		}

		return nil
	}

	err = fmt.Errorf("cannot find provider id for %s", id)

	return err
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
