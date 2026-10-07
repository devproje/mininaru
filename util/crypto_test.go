// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"database/sql"
	"os"
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

func TestSecretKeyMovesToCredentialsDB(t *testing.T) {
	var legacyKey string
	var encrypted string
	var decrypted string
	var db *sql.DB
	var storedKey string
	var info os.FileInfo

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	legacyKey = strings.Repeat("ab", secretKeyBytes)
	err = os.WriteFile(Path(secretKeyFile), []byte(legacyKey+"\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err = Encrypt("test-value")
	if err != nil {
		t.Fatal(err)
	}

	db, err = sql.Open("sqlite", databaseDSN(Path(credentialsDBFile)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.QueryRow("SELECT value FROM credentials WHERE name = 'master';").Scan(&storedKey)
	if err != nil {
		t.Fatal(err)
	}
	if storedKey != legacyKey {
		t.Fatal("legacy key was not copied into credentials.db")
	}

	info, err = os.Stat(Path(credentialsDBFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("credentials.db permissions = %v, want 0600", info.Mode().Perm())
	}

	_, err = os.Stat(Path(secretKeyFile))
	if !os.IsNotExist(err) {
		t.Fatal("legacy secret.key was not removed after migration")
	}

	decrypted, err = Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "test-value" {
		t.Fatal("credential DB key did not decrypt the existing value")
	}
}

func TestMismatchedSecretKeyFileIsKept(t *testing.T) {
	var info os.FileInfo

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, err = Encrypt("test-value")
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(Path(secretKeyFile), []byte(strings.Repeat("ab", secretKeyBytes)+"\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Encrypt("another-value")
	if err == nil {
		t.Fatal("expected a mismatched encryption key error")
	}

	info, err = os.Stat(Path(secretKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatal("mismatched secret.key was not preserved")
	}
}

func TestDecryptFailsWithoutCredentialsDB(t *testing.T) {
	var encrypted string

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err = Encrypt("test-value")
	if err != nil {
		t.Fatal(err)
	}

	err = os.Remove(Path(credentialsDBFile))
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected a missing encryption key error")
	}
}
