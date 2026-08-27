// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

func setupTestUpdateNaruPath(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
}

func setupTestUpdateAppVersion(t *testing.T, version string) {
	var previous string

	t.Helper()

	previous = util.AppVersion
	util.AppVersion = version

	t.Cleanup(func() { util.AppVersion = previous })
}

func buildTarGzFixture(t *testing.T, name string, content []byte) []byte {
	var buf bytes.Buffer
	var gz *gzip.Writer
	var tw *tar.Writer

	var err error

	t.Helper()

	gz = gzip.NewWriter(&buf)
	tw = tar.NewWriter(gz)

	err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(content))})
	if err != nil {
		t.Fatal(err)
	}

	_, err = tw.Write(content)
	if err != nil {
		t.Fatal(err)
	}

	tw.Close()
	gz.Close()

	return buf.Bytes()
}

func buildZipFixture(t *testing.T, name string, content []byte) []byte {
	var buf bytes.Buffer
	var zw *zip.Writer
	var entry io.Writer

	var err error

	t.Helper()

	zw = zip.NewWriter(&buf)

	entry, err = zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}

	_, err = entry.Write(content)
	if err != nil {
		t.Fatal(err)
	}

	zw.Close()

	return buf.Bytes()
}

func sha256Hex(buf []byte) string {
	var sum [32]byte

	sum = sha256.Sum256(buf)

	return hex.EncodeToString(sum[:])
}

func TestUpdateChecksumFindsTheMatchingEntry(t *testing.T) {
	var sums string
	var got string

	var err error

	sums = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  other.tar.gz\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  mininaru_v1_linux_amd64.tar.gz\n"

	got, err = updateChecksum([]byte(sums), "mininaru_v1_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.Repeat("b", 64) {
		t.Fatalf("got %q", got)
	}
}

func TestUpdateChecksumRejectsAMissingEntry(t *testing.T) {
	var err error

	_, err = updateChecksum([]byte("aaaa  something-else.tar.gz\n"), "mininaru_v1_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected an error for a missing entry")
	}
}

func TestUpdateDownloadArchiveRejectsAChecksumMismatch(t *testing.T) {
	var server *httptest.Server
	var dir string
	var content []byte
	var archive []byte

	var err error

	content = []byte("not the real binary")
	archive = buildTarGzFixture(t, "mininaru", content)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	dir = t.TempDir()

	_, err = updateDownloadArchive(t.Context(), server.URL, strings.Repeat("0", 64), dir)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v, want a checksum mismatch error", err)
	}
}

func TestUpdateStageDownloadsVerifiesAndExtractsTarGz(t *testing.T) {
	var server *httptest.Server
	var dir string
	var content []byte
	var archive []byte
	var staged string
	var got []byte

	var err error

	content = []byte("#!/bin/sh\necho fake-binary\n")
	archive = buildTarGzFixture(t, "mininaru", content)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	dir = t.TempDir()

	staged, err = updateStage(t.Context(), server.URL, sha256Hex(archive), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)

	got, err = os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("staged content = %q, want %q", got, content)
	}
}

func TestUpdateExtractZipPullsTheNamedBinaryOut(t *testing.T) {
	var server *httptest.Server
	var dir string
	var content []byte
	var archive []byte
	var archivePath string
	var staged string
	var got []byte

	var err error

	content = []byte("fake windows binary")
	archive = buildZipFixture(t, "mininaru.exe", content)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	dir = t.TempDir()

	archivePath, err = updateDownloadArchive(t.Context(), server.URL, sha256Hex(archive), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archivePath)

	staged, err = updateExtractZip(archivePath, dir, "mininaru.exe")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)

	got, err = os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("staged content = %q, want %q", got, content)
	}
}

func TestUpdateReplaceUnixMovesTheStagedFileIntoPlace(t *testing.T) {
	var dir string
	var target string
	var staged string
	var got []byte

	var err error

	dir = t.TempDir()
	target = filepath.Join(dir, "mininaru")
	staged = filepath.Join(dir, "mininaru.new")

	err = os.WriteFile(target, []byte("old"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(staged, []byte("new"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = updateReplaceUnix(staged, target)
	if err != nil {
		t.Fatal(err)
	}

	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target content = %q, want %q", got, "new")
	}
}

func TestUpdateReplaceWindowsRenamesTheRunningExecutableAside(t *testing.T) {
	var dir string
	var target string
	var staged string
	var got []byte

	var err error

	dir = t.TempDir()
	target = filepath.Join(dir, "mininaru.exe")
	staged = filepath.Join(dir, "mininaru.new")

	err = os.WriteFile(target, []byte("old"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(staged, []byte("new"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = updateReplaceWindows(staged, target)
	if err != nil {
		t.Fatal(err)
	}

	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target content = %q, want %q", got, "new")
	}

	if _, err = os.Stat(target + ".old"); err == nil {
		t.Fatal("the .old file should have been cleaned up once the rename succeeded")
	}
}

func newFakeGithub(t *testing.T, releases []release, assets map[string][]byte) *httptest.Server {
	var server *httptest.Server
	var previousBase string

	t.Helper()

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tag string
		var current release
		var found bool
		var buf []byte
		var ok bool

		switch {
		case r.URL.Path == "/repos/devproje/mininaru/releases":
			json.NewEncoder(w).Encode(releases)
		case strings.HasPrefix(r.URL.Path, "/repos/devproje/mininaru/releases/tags/"):
			tag = strings.TrimPrefix(r.URL.Path, "/repos/devproje/mininaru/releases/tags/")
			for _, current = range releases {
				if current.TagName == tag {
					found = true
					json.NewEncoder(w).Encode(current)
					break
				}
			}
			if !found {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			buf, ok = assets[r.URL.Path]
			if ok {
				w.Write(buf)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	previousBase = updateApiBase
	updateApiBase = server.URL

	t.Cleanup(func() {
		server.Close()
		updateApiBase = previousBase
	})

	return server
}

func TestUpdateLatestReleasePicksThePrereleaseOverAnOlderStableOne(t *testing.T) {
	var latest *release

	var err error

	newFakeGithub(t, []release{
		{TagName: "v1.0.0-alpha.2", Prerelease: true},
		{TagName: "v0.14.0", Prerelease: false},
	}, nil)

	latest, err = updateLatestRelease(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if latest.TagName != "v1.0.0-alpha.2" {
		t.Fatalf("latest.TagName = %q, want the newest prerelease, not the old stable line", latest.TagName)
	}
}

func TestUpdateTaggedReleaseFetchesASpecificTag(t *testing.T) {
	var got *release

	var err error

	newFakeGithub(t, []release{
		{TagName: "v1.0.0-alpha.2"},
		{TagName: "v1.0.0-alpha.1"},
	}, nil)

	got, err = updateTaggedRelease(t.Context(), "v1.0.0-alpha.1")
	if err != nil {
		t.Fatal(err)
	}
	if got.TagName != "v1.0.0-alpha.1" {
		t.Fatalf("got.TagName = %q, want %q", got.TagName, "v1.0.0-alpha.1")
	}
}

func TestUpdateExecuteCheckReportsBothVersionsWithoutInstalling(t *testing.T) {
	var cmd cobra.Command
	var output string

	var err error

	cmd.SetContext(t.Context())

	setupTestUpdateNaruPath(t)
	setupTestUpdateAppVersion(t, "v1.0.0-alpha.1")

	newFakeGithub(t, []release{{TagName: "v1.0.0-alpha.2", Prerelease: true}}, nil)

	updateTagRef = ""
	updateCheckRef = true
	updateForceRef = false
	t.Cleanup(func() { updateCheckRef = false })

	output = captureCommandStdout(t, func() {
		err = updateExecute(&cmd, nil)
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "v1.0.0-alpha.1") || !strings.Contains(output, "v1.0.0-alpha.2") {
		t.Fatalf("output = %q, want both the installed and latest versions", output)
	}
}

func TestUpdateExecuteSkipsAnAlreadyCurrentVersionWithoutForce(t *testing.T) {
	var cmd cobra.Command
	var output string

	var err error

	cmd.SetContext(t.Context())

	setupTestUpdateNaruPath(t)
	setupTestUpdateAppVersion(t, "v1.0.0-alpha.2")

	newFakeGithub(t, []release{{TagName: "v1.0.0-alpha.2", Prerelease: true}}, nil)

	updateTagRef = ""
	updateCheckRef = false
	updateForceRef = false

	output = captureCommandStdout(t, func() {
		err = updateExecute(&cmd, nil)
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "already running") {
		t.Fatalf("output = %q, want it to report already running the latest version", output)
	}
}

func captureCommandStdout(t *testing.T, run func()) string {
	var reader *os.File
	var writer *os.File
	var previous *os.File
	var buf bytes.Buffer

	var err error

	t.Helper()

	reader, writer, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	previous = os.Stdout
	os.Stdout = writer

	run()

	os.Stdout = previous
	writer.Close()

	_, err = io.Copy(&buf, reader)
	if err != nil {
		t.Fatal(err)
	}

	return buf.String()
}

func TestUpdateCheckSkippedForUpdateAndServe(t *testing.T) {
	var updateSub cobra.Command
	var serveSub cobra.Command
	var other cobra.Command

	updateSub.Use = "update"
	serveSub.Use = "serve"
	other.Use = "provider"

	if !updateCheckSkipped(&updateSub) {
		t.Fatal("expected the check to be skipped for the update command")
	}
	if !updateCheckSkipped(&serveSub) {
		t.Fatal("expected the check to be skipped for the serve command")
	}
	if updateCheckSkipped(&other) {
		t.Fatal("expected the check to run for an unrelated command")
	}
}

func TestUpdateCheckStartWritesTheCacheInTheBackground(t *testing.T) {
	var cmd cobra.Command
	var deadline time.Time
	var cache util.UpdateCache

	cmd.Use = "provider"

	setupTestUpdateNaruPath(t)
	setupTestUpdateAppVersion(t, "v1.0.0-alpha.1")

	newFakeGithub(t, []release{{TagName: "v1.0.0-alpha.9", Prerelease: true}}, nil)

	updateCheckStart(&cmd)

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cache = util.UpdateCacheRead()
		if cache.Tag == "v1.0.0-alpha.9" {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("update cache was never written, got %+v", cache)
}

func TestUpdateAssetNameMatchesTheReleaseWorkflowNaming(t *testing.T) {
	var name string

	name = updateAssetName("v1.0.0-alpha.2")

	if !strings.HasPrefix(name, "mininaru_v1.0.0-alpha.2_") {
		t.Fatalf("updateAssetName = %q, want it to start with mininaru_v1.0.0-alpha.2_", name)
	}
	if !strings.HasSuffix(name, updateAssetExt()) {
		t.Fatalf("updateAssetName = %q, want it to end with %q", name, updateAssetExt())
	}
}
