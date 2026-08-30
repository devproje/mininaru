// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

type ToolResult struct {
	Name   string `json:"name" xml:"name"`
	Status string `json:"status" xml:"status"`
}

type Result struct {
	XMLName   xml.Name     `json:"-" xml:"result"`
	SessionId string       `json:"session_id" xml:"session_id"`
	Content   string       `json:"content" xml:"content"`
	Tools     []ToolResult `json:"tools,omitempty" xml:"tool,omitempty"`
	Error     string       `json:"error,omitempty" xml:"error,omitempty"`
}

const (
	FormatString string = "string"
	FormatJSON   string = "json"
	FormatXML    string = "xml"
)

func ValidFormat(format string) bool {
	switch format {
	case "", FormatString, FormatJSON, FormatXML:
		return true
	}

	return false
}

func marshalResult(format string, result Result) ([]byte, error) {
	switch format {
	case FormatJSON:
		return json.MarshalIndent(result, "", "  ")
	case FormatXML:
		return xml.MarshalIndent(result, "", "  ")
	}

	return nil, fmt.Errorf("unknown format %q", format)
}
