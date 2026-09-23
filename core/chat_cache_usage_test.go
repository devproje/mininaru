// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"
)

func TestCachedTokensFromUsagePrefersTheOpenAIField(t *testing.T) {
	var usage openai.CompletionUsage
	var err error

	err = json.Unmarshal([]byte(`{"completion_tokens":1,"prompt_tokens":100,"total_tokens":101,"prompt_tokens_details":{"cached_tokens":40}}`), &usage)
	if err != nil {
		t.Fatal(err)
	}

	if cachedTokensFromUsage(usage) != 40 {
		t.Fatalf("cachedTokensFromUsage() = %d, want 40", cachedTokensFromUsage(usage))
	}
}

func TestCachedTokensFromUsageFallsBackToTheAnthropicField(t *testing.T) {
	var usage openai.CompletionUsage
	var err error

	err = json.Unmarshal([]byte(`{"completion_tokens":1,"prompt_tokens":100,"total_tokens":101,"cache_read_input_tokens":60}`), &usage)
	if err != nil {
		t.Fatal(err)
	}

	if cachedTokensFromUsage(usage) != 60 {
		t.Fatalf("cachedTokensFromUsage() = %d, want 60", cachedTokensFromUsage(usage))
	}
}

func TestCachedTokensFromUsageDefaultsToZero(t *testing.T) {
	var usage openai.CompletionUsage
	var err error

	err = json.Unmarshal([]byte(`{"completion_tokens":1,"prompt_tokens":100,"total_tokens":101}`), &usage)
	if err != nil {
		t.Fatal(err)
	}

	if cachedTokensFromUsage(usage) != 0 {
		t.Fatalf("cachedTokensFromUsage() = %d, want 0", cachedTokensFromUsage(usage))
	}
}
