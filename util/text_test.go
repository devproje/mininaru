// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateRunesPassesShortASCIIThrough(t *testing.T) {
	var result string

	result = TruncateRunes("hello", 10)
	if result != "hello" {
		t.Fatalf("result = %q, want %q", result, "hello")
	}
}

func TestTruncateRunesExactLengthBoundary(t *testing.T) {
	var result string

	result = TruncateRunes("hello", 5)
	if result != "hello" {
		t.Fatalf("result = %q, want %q", result, "hello")
	}
}

func TestTruncateRunesKeepsMultiByteRunesIntact(t *testing.T) {
	var input string
	var result string

	input = "안녕하세요"

	result = TruncateRunes(input, 3)
	if !utf8.ValidString(result) {
		t.Fatalf("result %q is not valid UTF-8", result)
	}
	if result != "안녕하" {
		t.Fatalf("result = %q, want %q", result, "안녕하")
	}
	if len(result) >= len(input) {
		t.Fatalf("result %q was not shorter than input %q", result, input)
	}
}
