// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/util"
)

func updateArchive(t *testing.T, entries map[string]string) []byte {
	var buf bytes.Buffer
	var gz *gzip.Writer
	var writer *tar.Writer
	var name string
	var body string

	var err error

	t.Helper()

	gz = gzip.NewWriter(&buf)
	writer = tar.NewWriter(gz)

	for name, body = range entries {
		err = writer.WriteHeader(&tar.Header{
			Name: name, Mode: 0755, Size: int64(len(body)), Typeflag: tar.TypeReg,
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err = writer.Write([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
	}

	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = gz.Close()
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func updateSum(payload []byte) string {
	var sum [sha256.Size]byte

	sum = sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}

func updateServer(t *testing.T, tag string, archive []byte, sums string) *httptest.Server {
	var srv *httptest.Server
	var assetName string

	t.Helper()

	assetName = fmt.Sprintf("mininaru_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"), strings.Contains(r.URL.Path, "/releases/tags/"):
			json.NewEncoder(w).Encode(release{
				TagName: tag,
				Assets: []releaseAsset{
					{Name: assetName, Url: "http://" + r.Host + "/download/" + assetName},
					{Name: updateSumsName, Url: "http://" + r.Host + "/download/" + updateSumsName},
				},
			})
		case strings.HasSuffix(r.URL.Path, updateSumsName):
			w.Write([]byte(sums))
		case strings.HasSuffix(r.URL.Path, assetName):
			w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	t.Cleanup(srv.Close)

	updateApiBase = srv.URL
	t.Cleanup(func() { updateApiBase = "https://api.github.com" })

	return srv
}

func TestUpdateChecksumReadsTheMatchingLine(t *testing.T) {
	var sums string
	var got string

	var err error

	sums = "aa  other.tar.gz\n" + strings.Repeat("b", 64) + "  wanted.tar.gz\n"

	got, err = updateChecksum([]byte(sums), "wanted.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.Repeat("b", 64) {
		t.Fatalf("unexpected checksum: %q", got)
	}

	_, err = updateChecksum([]byte(sums), "missing.tar.gz")
	if err == nil {
		t.Fatal("a missing entry was accepted")
	}

	_, err = updateChecksum([]byte("short  wanted.tar.gz\n"), "wanted.tar.gz")
	if err == nil {
		t.Fatal("a malformed checksum was accepted")
	}
}

func TestUpdateExtractTakesOnlyTheExecutable(t *testing.T) {
	var archive []byte
	var out bytes.Buffer

	var err error

	archive = updateArchive(t, map[string]string{
		"mininaru_linux_amd64/README.md": "docs",
		"mininaru_linux_amd64/mininaru":  "binary payload",
		"../evil":                        "escape",
	})

	err = updateExtract(bytes.NewReader(archive), &out)
	if err != nil {
		t.Fatal(err)
	}

	if out.String() != "binary payload" {
		t.Fatalf("extracted the wrong entry: %q", out.String())
	}
}

func TestUpdateExtractRejectsAnArchiveWithoutTheExecutable(t *testing.T) {
	var archive []byte
	var out bytes.Buffer

	var err error

	archive = updateArchive(t, map[string]string{"mininaru_linux_amd64/README.md": "docs"})

	err = updateExtract(bytes.NewReader(archive), &out)
	if err == nil {
		t.Fatal("an archive without the executable was accepted")
	}

	err = updateExtract(strings.NewReader("not gzip at all"), &out)
	if err == nil {
		t.Fatal("a non gzip payload was accepted")
	}
}

func TestUpdateStageRejectsAChecksumMismatch(t *testing.T) {
	var srv *httptest.Server
	var archive []byte
	var dir string
	var entries []os.DirEntry

	var err error

	archive = updateArchive(t, map[string]string{"mininaru_linux_amd64/mininaru": "binary payload"})
	srv = updateServer(t, "v9.9.9", archive, "")

	dir = t.TempDir()

	_, err = updateStage(context.Background(), srv.URL+"/download/mininaru_v9.9.9_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz",
		strings.Repeat("0", 64), dir)
	if err == nil {
		t.Fatal("a mismatched checksum was accepted")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the staged file was left behind: %v", entries)
	}
}

func TestUpdateStageAcceptsAMatchingChecksum(t *testing.T) {
	var srv *httptest.Server
	var archive []byte
	var staged string
	var buf []byte

	var err error

	archive = updateArchive(t, map[string]string{"mininaru_linux_amd64/mininaru": "binary payload"})
	srv = updateServer(t, "v9.9.9", archive, "")

	staged, err = updateStage(context.Background(), srv.URL+"/download/mininaru_v9.9.9_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz",
		updateSum(archive), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	buf, err = os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "binary payload" {
		t.Fatalf("staged the wrong content: %q", string(buf))
	}
}

func TestUpdateReplaceSwapsTheFileAndKeepsItExecutable(t *testing.T) {
	var dir string
	var target string
	var staged string
	var info os.FileInfo
	var buf []byte

	var err error

	dir = t.TempDir()
	target = filepath.Join(dir, "mininaru")
	staged = filepath.Join(dir, "mininaru.new")

	err = os.WriteFile(target, []byte("old"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(staged, []byte("new"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = updateReplace(staged, target)
	if err != nil {
		t.Fatal(err)
	}

	buf, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "new" {
		t.Fatalf("the target was not replaced: %q", string(buf))
	}

	info, err = os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("the replacement is not executable: %v", info.Mode())
	}

	if _, err = os.Stat(staged); !os.IsNotExist(err) {
		t.Fatal("the staged file outlived the rename")
	}
}

func TestUpdateAssetNameMatchesTheReleaseLayout(t *testing.T) {
	var name string

	name = updateAssetName("v0.3.0")
	if name != "mininaru_v0.3.0_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz" {
		t.Fatalf("unexpected asset name: %q", name)
	}
}

func TestUpdateFindAssetReportsAMissingBuild(t *testing.T) {
	var current release

	var err error

	current = release{TagName: "v0.3.0", Assets: []releaseAsset{{Name: "mininaru_v0.3.0_plan9_386.tar.gz"}}}

	_, err = updateFindAsset(&current, updateAssetName("v0.3.0"))
	if err == nil {
		t.Fatal("a missing asset was accepted")
	}
	if !strings.Contains(err.Error(), "v0.3.0") {
		t.Fatalf("the error does not name the release: %v", err)
	}
}

func updateNoticeSetup(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	config.Client.Update.Check = true
	util.AppVersion = "v0.1.0"

	t.Cleanup(func() { util.AppVersion = devVersion })
}

func TestUpdateNoticeAppearsOnlyForANewerTag(t *testing.T) {
	var notice string

	updateNoticeSetup(t)

	if updateNotice() != "" {
		t.Fatal("a notice appeared without a cache")
	}

	updateCacheWrite("v0.1.0")
	if updateNotice() != "" {
		t.Fatal("a notice appeared for the running version")
	}

	updateCacheWrite("v0.2.0")

	notice = updateNotice()
	if !strings.Contains(notice, "v0.2.0") {
		t.Fatalf("the notice does not name the new version: %q", notice)
	}
}

func TestUpdateNoticeStaysQuietForDevAndWhenDisabled(t *testing.T) {
	updateNoticeSetup(t)
	updateCacheWrite("v0.2.0")

	util.AppVersion = devVersion
	if updateNotice() != "" {
		t.Fatal("a dev build was told to update")
	}

	util.AppVersion = "v0.1.0"
	config.Client.Update.Check = false
	t.Cleanup(func() { config.Client.Update.Check = true })

	if updateNotice() != "" {
		t.Fatal("the notice ignored the disabled setting")
	}

	config.Client.Update.Check = true
	t.Setenv(config.NoUpdateCheckEnv, "1")

	if updateNotice() != "" {
		t.Fatal("the notice ignored the environment override")
	}
}

func TestUpdateCacheSurvivesACorruptFile(t *testing.T) {
	var cached updateCache

	var err error

	updateNoticeSetup(t)

	err = os.WriteFile(updateCachePath(), []byte("{not json"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cached = updateCacheRead()
	if cached.Tag != "" || cached.CheckedAt != 0 {
		t.Fatalf("a corrupt cache was trusted: %+v", cached)
	}
}

func TestUpdateCacheRecordsTheCheckTime(t *testing.T) {
	var cached updateCache

	updateNoticeSetup(t)
	updateCacheWrite("v0.2.0")

	cached = updateCacheRead()
	if cached.Tag != "v0.2.0" {
		t.Fatalf("unexpected cached tag: %q", cached.Tag)
	}
	if time.Since(time.Unix(cached.CheckedAt, 0)) > time.Minute {
		t.Fatalf("the check time was not recorded: %d", cached.CheckedAt)
	}
}

func TestUpdateCheckSkippedForLongRunningCommands(t *testing.T) {
	if !updateCheckSkipped(updateCmd) {
		t.Fatal("the update command should not run a background check")
	}
	if !updateCheckSkipped(daemonReload) {
		t.Fatal("daemon subcommands should not run a background check")
	}
	if !updateCheckSkipped(serve) {
		t.Fatal("serve should not run a background check")
	}
	if updateCheckSkipped(skillListCmd) {
		t.Fatal("ordinary commands should run a background check")
	}
}

func TestUpdateCacheHoldsTheLatestTagNotTheInstalledOne(t *testing.T) {
	var cached updateCache

	var err error

	updateNoticeSetup(t)
	updateServer(t, "v9.9.9", updateArchive(t, map[string]string{"x/mininaru": "payload"}), "")

	updateCacheWrite("v9.9.9")

	updateTagRef = "v0.1.0"
	updateCheckRef = true
	t.Cleanup(func() { updateTagRef = ""; updateCheckRef = false })

	updateCmd.SetContext(context.Background())

	err = updateExecute(updateCmd, nil)
	if err != nil {
		t.Fatal(err)
	}

	cached = updateCacheRead()
	if cached.Tag != "v9.9.9" {
		t.Fatalf("an explicit --tag overwrote the latest tag in the cache: %q", cached.Tag)
	}
}

func TestUpdateFetchReleaseReadsTheTag(t *testing.T) {
	var srv *httptest.Server
	var latest *release

	var err error

	srv = updateServer(t, "v9.9.9", updateArchive(t, map[string]string{"x/mininaru": "payload"}), "")
	_ = srv

	latest, err = updateFetchRelease(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if latest.TagName != "v9.9.9" {
		t.Fatalf("unexpected tag: %q", latest.TagName)
	}

	_, err = updateFindAsset(latest, updateAssetName("v9.9.9"))
	if err != nil {
		t.Fatal(err)
	}
}
