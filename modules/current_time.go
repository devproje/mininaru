package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func CurrentTime() Def {
	return Def{
		Name:        "current_time",
		Description: "Return the current date and time for an IANA timezone.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"timezone": map[string]any{
					"type":        "string",
					"description": "IANA timezone such as Asia/Seoul or UTC. Defaults to UTC.",
				},
			},
			"additionalProperties": false,
		},
		Permission: PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Timezone string `json:"timezone"`
			}
			var location *time.Location
			var now time.Time
			var buf []byte

			var err error

			if arguments != "" {
				err = json.Unmarshal([]byte(arguments), &payload)
				if err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}
			}
			if payload.Timezone == "" {
				payload.Timezone = "UTC"
			}

			location, err = time.LoadLocation(payload.Timezone)
			if err != nil {
				return "", fmt.Errorf("invalid timezone %q", payload.Timezone)
			}

			now = time.Now().In(location)
			buf, err = json.Marshal(map[string]string{
				"timezone": payload.Timezone,
				"datetime": now.Format(time.RFC3339),
			})
			if err != nil {
				return "", err
			}

			return string(buf), nil
		},
	}
}
