// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package ask_question

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devproje/mininaru/modules"
)

func Question(ask modules.AskFunc) modules.Tool {
	return modules.Tool{
		Name:        "ask_user_question",
		Description: "Ask the human a question mid-conversation and wait for their answer. Provide options for a multiple-choice question; the human may still type their own answer instead of picking one.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{"type": "string"},
				"options":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required":             []string{"question"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Question string   `json:"question"`
				Options  []string `json:"options"`
			}
			var answer string

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Question == "" {
				return "", fmt.Errorf("question is required")
			}
			if ask == nil {
				return "", fmt.Errorf("no interactive user available to answer this question")
			}

			answer, err = ask(ctx, payload.Question, payload.Options)
			if err != nil {
				return "", err
			}

			return answer, nil
		},
	}
}

func Tools(ask modules.AskFunc) []modules.Tool {
	return []modules.Tool{Question(ask)}
}
