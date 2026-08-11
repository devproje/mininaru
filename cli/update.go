// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	stdhash "hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

type releaseAsset struct {
	Name string `json:"name"`
	Url  string `json:"browser_download_url"`
}

type release struct {
	TagName    string         `json:"tag_name"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type updateCache struct {
	Tag       string `json:"tag"`
	CheckedAt int64  `json:"checked_at"`
}

const updateRepo = "devproje/mininaru"

const updateBinaryName = "mininaru"

const updateSumsName = "SHA256SUMS"

const updateCacheFile = "update.json"

const updateCacheTTL = 24 * time.Hour

const maxUpdateBinary = 64 << 20

const maxUpdateSums = 1 << 20

const maxUpdateRelease = 4 << 20

const devVersion = "dev"

var updateApiBase string = "https://api.github.com"

var updateClient *http.Client = &http.Client{Timeout: 60 * time.Second}

var (
	updateTagRef       string
	updateCheckRef     bool
	updateForceRef     bool
	updateNoRestartRef bool
)

var updateCmd *cobra.Command = &cobra.Command{
	Use:   "update",
	Short: "download the latest release and replace this executable",
	Long: `Replace the running mininaru executable with a published release build.

The archive is verified against the release's SHA256SUMS before anything is
replaced, and the new file is moved into place atomically, so a failed update
leaves the current executable untouched. If the systemd user daemon is
installed it is restarted afterwards.

Linux and macOS only; on Windows, download the zip from the releases page.`,
	Example: `  mininaru update
  mininaru update --check
  mininaru update --tag v0.3.0`,
	Args: usageArgs(cobra.NoArgs),
	RunE: updateExecute,
}

func updateSupported() error {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		return nil
	}

	return configErrorf("`mininaru update` supports linux and darwin, not %s: download the release from https://github.com/%s/releases",
		runtime.GOOS, updateRepo)
}

func updateAssetName(tag string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", updateBinaryName, tag, runtime.GOOS, runtime.GOARCH)
}

func updateGet(ctx context.Context, url string) (*http.Response, error) {
	var request *http.Request
	var response *http.Response
	var token string

	var err error

	request, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", updateBinaryName+"/"+util.AppVersion)

	token = os.Getenv("GITHUB_TOKEN")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err = updateClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusOK {
		return response, nil
	}

	response.Body.Close()

	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("github refused the request with %s, which usually means the rate limit is exhausted: set GITHUB_TOKEN or retry later", response.Status)
	}

	return nil, fmt.Errorf("github answered %s for %s", response.Status, url)
}

func updateFetchRelease(ctx context.Context, tag string) (*release, error) {
	var url string
	var response *http.Response
	var current release

	var err error

	url = updateApiBase + "/repos/" + updateRepo + "/releases/latest"
	if tag != "" {
		url = updateApiBase + "/repos/" + updateRepo + "/releases/tags/" + tag
	}

	response, err = updateGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = json.NewDecoder(io.LimitReader(response.Body, maxUpdateRelease)).Decode(&current)
	if err != nil {
		return nil, fmt.Errorf("reading the release failed: %w", err)
	}

	if current.TagName == "" {
		return nil, fmt.Errorf("the release has no tag name")
	}

	return &current, nil
}

func updateFindAsset(current *release, name string) (string, error) {
	var asset releaseAsset

	for _, asset = range current.Assets {
		if asset.Name != name {
			continue
		}

		return asset.Url, nil
	}

	return "", fmt.Errorf("release %s has no asset named %s", current.TagName, name)
}

func updateChecksum(sums []byte, name string) (string, error) {
	var line string
	var fields []string

	for _, line = range strings.Split(string(sums), "\n") {
		fields = strings.Fields(line)
		if len(fields) != 2 || fields[1] != name {
			continue
		}

		if len(fields[0]) != sha256.Size*2 {
			return "", fmt.Errorf("%s holds a malformed checksum for %s", updateSumsName, name)
		}

		return strings.ToLower(fields[0]), nil
	}

	return "", fmt.Errorf("%s has no entry for %s", updateSumsName, name)
}

func updateDownloadSums(ctx context.Context, url string) ([]byte, error) {
	var response *http.Response
	var buf []byte

	var err error

	response, err = updateGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	buf, err = io.ReadAll(io.LimitReader(response.Body, maxUpdateSums))
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func updateExtract(archive io.Reader, out io.Writer) error {
	var reader *gzip.Reader
	var entries *tar.Reader
	var header *tar.Header
	var written int64

	var err error

	reader, err = gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("the archive is not gzip: %w", err)
	}
	defer reader.Close()

	entries = tar.NewReader(reader)

	for {
		header, err = entries.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading the archive failed: %w", err)
		}

		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != updateBinaryName {
			continue
		}

		written, err = io.Copy(out, io.LimitReader(entries, maxUpdateBinary+1))
		if err != nil {
			return err
		}
		if written > maxUpdateBinary {
			return fmt.Errorf("the executable in the archive exceeds %d bytes", maxUpdateBinary)
		}
		if written == 0 {
			return fmt.Errorf("the executable in the archive is empty")
		}

		return nil
	}

	return fmt.Errorf("the archive has no %s executable", updateBinaryName)
}

func updateStage(ctx context.Context, url, want, dir string) (string, error) {
	var response *http.Response
	var hasher stdhash.Hash
	var reader io.Reader
	var staged *os.File
	var got string

	var err error

	hasher = sha256.New()

	response, err = updateGet(ctx, url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	staged, err = os.CreateTemp(dir, updateBinaryName+".new")
	if err != nil {
		return "", fmt.Errorf("cannot stage the download next to the current executable in %s: %w", dir, err)
	}

	reader = io.TeeReader(response.Body, hasher)

	err = updateExtract(reader, staged)
	if err == nil {
		_, err = io.Copy(io.Discard, reader)
	}

	staged.Close()

	if err != nil {
		os.Remove(staged.Name())

		return "", err
	}

	got = hex.EncodeToString(hasher.Sum(nil))
	if got != want {
		os.Remove(staged.Name())

		return "", fmt.Errorf("checksum mismatch for the download: expected %s, got %s", want, got)
	}

	return staged.Name(), nil
}

func updateTarget() (string, error) {
	var target string

	var err error

	target, err = os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(target)
}

func updateReplace(staged, target string) error {
	var err error

	err = os.Chmod(staged, 0755)
	if err != nil {
		return err
	}

	err = os.Rename(staged, target)
	if err != nil {
		os.Remove(staged)

		return fmt.Errorf("replacing %s failed: %w", target, err)
	}

	return nil
}

func updateCachePath() string {
	return util.Path(updateCacheFile)
}

func updateCacheRead() updateCache {
	var current updateCache
	var buf []byte

	var err error

	buf, err = os.ReadFile(updateCachePath())
	if err != nil {
		return current
	}

	err = json.Unmarshal(buf, &current)
	if err != nil {
		return updateCache{}
	}

	return current
}

func updateCacheWrite(tag string) {
	var buf []byte

	var err error

	buf, err = json.MarshalIndent(updateCache{Tag: tag, CheckedAt: time.Now().Unix()}, "", "    ")
	if err != nil {
		return
	}

	err = util.WriteFileAtomic(updateCachePath(), buf, 0600)
	if err != nil {
		util.Log.Debug("caching the update check failed", "error", err)
	}
}

func updateNotice() string {
	var current updateCache

	if util.AppVersion == devVersion || !config.UpdateCheckEnabled() {
		return ""
	}

	current = updateCacheRead()
	if current.Tag == "" || current.Tag == util.AppVersion {
		return ""
	}

	return fmt.Sprintf("a newer version is available: %s (run `mininaru update`)", current.Tag)
}

func updateCheckSkipped(cmd *cobra.Command) bool {
	var current *cobra.Command

	for current = cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "update", "serve", "daemon":
			return true
		}
	}

	return false
}

func updateCheckStart(cmd *cobra.Command) {
	var cached updateCache

	if util.AppVersion == devVersion || !config.UpdateCheckEnabled() || updateCheckSkipped(cmd) {
		return
	}

	cached = updateCacheRead()
	if time.Since(time.Unix(cached.CheckedAt, 0)) < updateCacheTTL {
		return
	}

	go func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var latest *release

		var err error

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		latest, err = updateFetchRelease(ctx, "")
		if err != nil {
			util.Log.Debug("the background update check failed", "error", err)

			return
		}

		updateCacheWrite(latest.TagName)
	}()
}

func updateExecute(cmd *cobra.Command, args []string) error {
	var latest *release
	var target string
	var assetName string
	var assetUrl string
	var sumsUrl string
	var sums []byte
	var want string
	var staged string
	var installed bool

	var err error

	latest, err = updateFetchRelease(cmd.Context(), updateTagRef)
	if err != nil {
		return err
	}

	if updateTagRef == "" {
		updateCacheWrite(latest.TagName)
	}

	if updateCheckRef {
		uiOk("installed: %s", util.AppVersion)
		uiOk("latest:    %s", latest.TagName)

		if latest.TagName == util.AppVersion {
			uiNote("already up to date")

			return nil
		}

		uiNote("run `mininaru update` to install %s", latest.TagName)

		return nil
	}

	err = updateSupported()
	if err != nil {
		return err
	}

	if latest.TagName == util.AppVersion && !updateForceRef {
		uiOk("already running %s", util.AppVersion)
		uiNote("pass --force to reinstall it")

		return nil
	}

	target, err = updateTarget()
	if err != nil {
		return err
	}

	assetName = updateAssetName(latest.TagName)

	assetUrl, err = updateFindAsset(latest, assetName)
	if err != nil {
		return err
	}

	sumsUrl, err = updateFindAsset(latest, updateSumsName)
	if err != nil {
		return err
	}

	sums, err = updateDownloadSums(cmd.Context(), sumsUrl)
	if err != nil {
		return err
	}

	want, err = updateChecksum(sums, assetName)
	if err != nil {
		return err
	}

	uiNote("downloading %s", assetName)

	staged, err = updateStage(cmd.Context(), assetUrl, want, filepath.Dir(target))
	if err != nil {
		return err
	}

	err = updateReplace(staged, target)
	if err != nil {
		return err
	}

	uiOk("updated %s to %s", target, latest.TagName)

	if updateNoRestartRef {
		return nil
	}

	installed, err = daemonInstalled()
	if err != nil {
		return err
	}
	if !installed {
		return nil
	}

	err = daemonRestart(cmd.Context())
	if err != nil {
		return fmt.Errorf("the executable was updated but restarting the daemon failed: %w", err)
	}

	uiOk("restarted %s", daemonUnitName)

	return nil
}

func init() {
	updateCmd.Flags().StringVar(&updateTagRef, "tag", "", "release tag to install, defaults to the latest release")
	updateCmd.Flags().BoolVar(&updateCheckRef, "check", false, "report the installed and latest versions without installing")
	updateCmd.Flags().BoolVar(&updateForceRef, "force", false, "reinstall even when the latest version is already running")
	updateCmd.Flags().BoolVar(&updateNoRestartRef, "no-restart", false, "leave the systemd user daemon alone after updating")
}
