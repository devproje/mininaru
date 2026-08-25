// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"strings"

	"github.com/devproje/mininaru/modules"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxToolNameLength = 64

const toolNameSeparator = "__"

func schemaObject(schema any) map[string]any {
	var object map[string]any
	var ok bool
	var buf []byte

	var err error

	object, ok = schema.(map[string]any)
	if !ok {
		if schema != nil {
			buf, err = json.Marshal(schema)
			if err == nil {
				err = json.Unmarshal(buf, &object)
			}
		}
		if object == nil || err != nil {
			return map[string]any{"type": "object", "properties": map[string]any{}}
		}
	}

	if object["type"] == nil {
		object["type"] = "object"
	}

	return object
}

func nameAllowed(char byte) bool {
	if char >= 'a' && char <= 'z' {
		return true
	}
	if char >= 'A' && char <= 'Z' {
		return true
	}
	if char >= '0' && char <= '9' {
		return true
	}

	return char == '_' || char == '-'
}

func fnv32a(value string) uint32 {
	var digest hash.Hash32

	digest = fnv.New32a()
	digest.Write([]byte(value))

	return digest.Sum32() & 0xffffff
}

func qualifiedName(server, tool string) string {
	var raw string
	var index int
	var sanitized strings.Builder
	var digest string

	raw = server + toolNameSeparator + tool

	for index = 0; index < len(raw); index++ {
		if nameAllowed(raw[index]) {
			sanitized.WriteByte(raw[index])
			continue
		}

		sanitized.WriteByte('_')
	}

	if sanitized.Len() <= maxToolNameLength {
		return sanitized.String()
	}

	digest = fmt.Sprintf("%06x", fnv32a(raw))

	return sanitized.String()[:maxToolNameLength-len(digest)-1] + "_" + digest
}

func annotationPermission(annotations *mcpsdk.ToolAnnotations) modules.Permission {
	if annotations == nil {
		return modules.PermissionDangerous
	}
	if annotations.ReadOnlyHint {
		return modules.PermissionSafe
	}

	return modules.PermissionDangerous
}

func parsePermission(value string) (modules.Permission, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "safe":
		return modules.PermissionSafe, true
	case "dangerous":
		return modules.PermissionDangerous, true
	}

	return modules.PermissionDangerous, false
}

func overridePermission(entry *Server, tool string, derived modules.Permission) modules.Permission {
	var value string
	var ok bool
	var permission modules.Permission

	value, ok = entry.ToolPermission[tool]
	if ok {
		permission, ok = parsePermission(value)
		if ok {
			return permission
		}
	}

	if entry.Permission != "" {
		permission, ok = parsePermission(entry.Permission)
		if ok {
			return permission
		}
	}

	return derived
}

func contentText(content []mcpsdk.Content) string {
	var cur mcpsdk.Content
	var text *mcpsdk.TextContent
	var ok bool
	var parts []string
	var image *mcpsdk.ImageContent
	var audio *mcpsdk.AudioContent
	var buf []byte

	for _, cur = range content {
		text, ok = cur.(*mcpsdk.TextContent)
		if ok {
			parts = append(parts, text.Text)
			continue
		}

		image, ok = cur.(*mcpsdk.ImageContent)
		if ok {
			parts = append(parts, fmt.Sprintf("[image content omitted: %s, %d bytes]", image.MIMEType, len(image.Data)))
			continue
		}

		audio, ok = cur.(*mcpsdk.AudioContent)
		if ok {
			parts = append(parts, fmt.Sprintf("[audio content omitted: %s, %d bytes]", audio.MIMEType, len(audio.Data)))
			continue
		}

		buf, _ = cur.MarshalJSON()
		parts = append(parts, string(buf))
	}

	return strings.Join(parts, "\n")
}

func resultText(result *mcpsdk.CallToolResult) (string, error) {
	var text string
	var buf []byte

	var err error

	if result == nil {
		return "", errors.New("tool call returned no result")
	}

	text = contentText(result.Content)

	if text == "" && result.StructuredContent != nil {
		buf, err = json.Marshal(result.StructuredContent)
		if err != nil {
			return "", err
		}

		text = string(buf)
	}

	if result.IsError {
		if text == "" {
			text = "tool call failed"
		}

		return "", errors.New(text)
	}

	return text, nil
}
