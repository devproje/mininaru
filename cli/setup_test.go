package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupEnvFileCreatesAPrivateFile(t *testing.T) {
	var path string
	var info os.FileInfo
	var buf []byte

	var err error

	fakeSession(t, "sk-secret\n")

	path = filepath.Join(t.TempDir(), "nested", "env")

	err = setupEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("env file mode = %v, want 0600", info.Mode().Perm())
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(buf)) != apiKeyEnv+"=sk-secret" {
		t.Fatalf("env file holds %q", string(buf))
	}
}

func TestSetupEnvFileGeneratesAKeyWhenLeftEmpty(t *testing.T) {
	var path string
	var buf []byte
	var value string

	var err error

	fakeSession(t, "\n")

	path = filepath.Join(t.TempDir(), "env")

	err = setupEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	value = strings.TrimPrefix(strings.TrimSpace(string(buf)), apiKeyEnv+"=")
	if len(value) != 48 {
		t.Fatalf("generated key %q is %d chars, want 48 hex chars", value, len(value))
	}
}

func TestSetupEnvFileKeepsAValidFile(t *testing.T) {
	var path string
	var buf []byte

	var err error

	fakeSession(t, "sk-should-not-be-used\n")

	path = filepath.Join(t.TempDir(), "env")

	err = os.WriteFile(path, []byte("# existing\n"+apiKeyEnv+"=sk-original\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = setupEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(buf), "sk-original") || !strings.Contains(string(buf), "# existing") {
		t.Fatalf("a valid env file was rewritten: %q", string(buf))
	}
}

func TestSetupEnvFileRefusesToClobberAFileWithoutTheKey(t *testing.T) {
	var path string
	var buf []byte

	var err error

	fakeSession(t, "sk-secret\n")

	path = filepath.Join(t.TempDir(), "env")

	err = os.WriteFile(path, []byte("OTHER_SETTING=1\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = setupEnvFile(path)
	if err == nil {
		t.Fatal("expected an error rather than overwriting an unrelated env file")
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(buf)) != "OTHER_SETTING=1" {
		t.Fatalf("existing content was destroyed: %q", string(buf))
	}
}
