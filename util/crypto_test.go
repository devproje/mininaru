// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	var encrypted string
	var decrypted string

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err = Encrypt("sk-super-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, encPrefix) {
		t.Fatalf("encrypted value %q missing prefix %q", encrypted, encPrefix)
	}
	if encrypted == "sk-super-secret" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}

	decrypted, err = Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "sk-super-secret" {
		t.Fatalf("decrypted = %q, want %q", decrypted, "sk-super-secret")
	}
}

func TestEncryptDecryptEmptyString(t *testing.T) {
	var encrypted string
	var decrypted string

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err = Encrypt("")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted != "" {
		t.Fatalf("encrypted = %q, want empty string", encrypted)
	}

	decrypted, err = Decrypt("")
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "" {
		t.Fatalf("decrypted = %q, want empty string", decrypted)
	}
}

func TestDecryptPassesThroughLegacyPlaintext(t *testing.T) {
	var decrypted string

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err = Decrypt("plain-old-key")
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "plain-old-key" {
		t.Fatalf("decrypted = %q, want %q", decrypted, "plain-old-key")
	}
}
