// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package client

import (
	"testing"
	"unicode/utf8"
)

func TestDropLastRuneOnEmptyBuffer(t *testing.T) {
	var result []byte

	result = dropLastRune(nil)
	if len(result) != 0 {
		t.Fatalf("result = %q, want empty", result)
	}
}

func TestDropLastRuneOnSingleASCIIByte(t *testing.T) {
	var result []byte

	result = dropLastRune([]byte("a"))
	if len(result) != 0 {
		t.Fatalf("result = %q, want empty", result)
	}
}

func TestDropLastRuneOnMultiByteRune(t *testing.T) {
	var buf []byte
	var result []byte

	buf = append([]byte("hi"), "안"...)

	result = dropLastRune(buf)
	if !utf8.Valid(result) {
		t.Fatalf("result %q is not valid UTF-8", result)
	}
	if string(result) != "hi" {
		t.Fatalf("result = %q, want %q", result, "hi")
	}
}
