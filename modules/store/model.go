// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package store

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
)

type ModelData struct {
	Models     []string
	ExpiresAt  time.Time
	generation uint64

	sync.RWMutex
}

const modelCacheTTL = 5 * time.Minute

var cacheModels ModelData
var modelRefreshMu sync.Mutex

func cachedModels() ([]string, bool) {
	cacheModels.RLock()
	defer cacheModels.RUnlock()

	if !time.Now().Before(cacheModels.ExpiresAt) {
		return nil, false
	}

	return slices.Clone(cacheModels.Models), true
}

func InvalidateModels() {
	cacheModels.Lock()
	cacheModels.Models = nil
	cacheModels.ExpiresAt = time.Time{}
	cacheModels.generation++
	cacheModels.Unlock()
}

func fetchModels(ctx context.Context) ([]string, error) {
	var prov []Provider

	var cur Provider
	var opts []option.RequestOption
	var ai openai.Client

	var page *pagination.Page[openai.Model]
	var model openai.Model

	var models []string

	var err error

	prov, err = GetProviders()
	if err != nil {
		return nil, err
	}

	for _, cur = range prov {
		opts = []option.RequestOption{}
		if cur.ApiKey != "" {
			opts = append(opts, option.WithAPIKey(cur.ApiKey))
		}

		if cur.BaseUrl != "" {
			opts = append(opts, option.WithBaseURL(cur.BaseUrl))
		}

		ai = openai.NewClient(opts...)
		page, err = ai.Models.List(ctx)
		if err != nil {
			return nil, err
		}

		for _, model = range page.Data {
			models = append(models, fmt.Sprintf("%s:%s", cur.Name, model.ID))
		}
	}

	return models, nil
}

func ListModel(ctx context.Context) ([]string, error) {
	var models []string
	var ok bool

	var generation uint64

	var err error

	models, ok = cachedModels()
	if ok {
		return models, nil
	}

	modelRefreshMu.Lock()
	defer modelRefreshMu.Unlock()

	for {
		models, ok = cachedModels()
		if ok {
			return models, nil
		}

		cacheModels.RLock()
		generation = cacheModels.generation
		cacheModels.RUnlock()

		models, err = fetchModels(ctx)
		if err != nil {
			return nil, err
		}

		cacheModels.Lock()
		if cacheModels.generation != generation {
			cacheModels.Unlock()
			continue
		}

		cacheModels.Models = slices.Clone(models)
		cacheModels.ExpiresAt = time.Now().Add(modelCacheTTL)
		cacheModels.Unlock()

		return models, nil
	}
}

func ValidateModel(ctx context.Context, model string) (bool, error) {
	var models []string
	var ok bool

	var err error

	models, err = ListModel(ctx)
	if err != nil {
		return false, err
	}

	ok = slices.Contains(models, model)

	return ok, nil
}
