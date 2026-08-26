// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"archive/tar"
	"archive/zip"
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

const updateRepo = "devproje/mininaru"

const updateBinaryName = "mininaru"

const updateSumsName = "SHA256SUMS"

const maxUpdateBinary = 64 << 20

const maxUpdateSums = 1 << 20

const maxUpdateRelease = 4 << 20

const maxUpdateReleaseList = 16 << 20

var updateApiBase string = "https://api.github.com"

var updateClient *http.Client = &http.Client{Timeout: 60 * time.Second}

var (
	updateTagRef   string
	updateCheckRef bool
	updateForceRef bool
)

var updateCmd *cobra.Command = &cobra.Command{
	Use:   "update",
	Short: "download the latest release and replace this executable",
	Long: `Replace the running mininaru executable with a published release build.

The archive is verified against the release's SHA256SUMS before anything is
replaced, and the new file is moved into place atomically.`,
	Example: `  mininaru update
  mininaru update --check
  mininaru update --tag v1.0.0-alpha.2`,
	Args: cobra.NoArgs,
	RunE: updateExecute,
}

func updateAssetExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}

	return ".tar.gz"
}

func updateBinaryFile() string {
	if runtime.GOOS == "windows" {
		return updateBinaryName + ".exe"
	}

	return updateBinaryName
}

func updateAssetName(tag string) string {
	return fmt.Sprintf("%s_%s_%s_%s%s", updateBinaryName, tag, runtime.GOOS, runtime.GOARCH, updateAssetExt())
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

func updateLatestRelease(ctx context.Context) (*release, error) {
	var url string
	var response *http.Response
	var list []release

	var err error

	url = updateApiBase + "/repos/" + updateRepo + "/releases"

	response, err = updateGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = json.NewDecoder(io.LimitReader(response.Body, maxUpdateReleaseList)).Decode(&list)
	if err != nil {
		return nil, fmt.Errorf("reading the release list failed: %w", err)
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("repository %s has no releases", updateRepo)
	}

	return &list[0], nil
}

func updateTaggedRelease(ctx context.Context, tag string) (*release, error) {
	var url string
	var response *http.Response
	var current release

	var err error

	url = updateApiBase + "/repos/" + updateRepo + "/releases/tags/" + tag

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

func updateFetchRelease(ctx context.Context, tag string) (*release, error) {
	if tag == "" {
		return updateLatestRelease(ctx)
	}

	return updateTaggedRelease(ctx, tag)
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

func updateDownloadArchive(ctx context.Context, url, want, dir string) (string, error) {
	var response *http.Response
	var hasher stdhash.Hash
	var reader io.Reader
	var archive *os.File
	var got string

	var err error

	hasher = sha256.New()

	response, err = updateGet(ctx, url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	archive, err = os.CreateTemp(dir, updateBinaryName+".archive")
	if err != nil {
		return "", fmt.Errorf("cannot stage the download in %s: %w", dir, err)
	}

	reader = io.TeeReader(io.LimitReader(response.Body, maxUpdateBinary+1<<20), hasher)

	_, err = io.Copy(archive, reader)
	archive.Close()
	if err != nil {
		os.Remove(archive.Name())

		return "", err
	}

	got = hex.EncodeToString(hasher.Sum(nil))
	if got != want {
		os.Remove(archive.Name())

		return "", fmt.Errorf("checksum mismatch for the download: expected %s, got %s", want, got)
	}

	return archive.Name(), nil
}

func updateExtractTarGz(archivePath, dir, binaryName string) (string, error) {
	var archive *os.File
	var reader *gzip.Reader
	var entries *tar.Reader
	var header *tar.Header
	var staged *os.File
	var written int64

	var err error

	archive, err = os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	reader, err = gzip.NewReader(archive)
	if err != nil {
		return "", fmt.Errorf("the archive is not gzip: %w", err)
	}
	defer reader.Close()

	entries = tar.NewReader(reader)

	for {
		header, err = entries.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading the archive failed: %w", err)
		}

		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}

		staged, err = os.CreateTemp(dir, updateBinaryName+".new")
		if err != nil {
			return "", fmt.Errorf("cannot stage the executable in %s: %w", dir, err)
		}

		written, err = io.Copy(staged, io.LimitReader(entries, maxUpdateBinary+1))
		staged.Close()
		if err != nil {
			os.Remove(staged.Name())

			return "", err
		}
		if written > maxUpdateBinary {
			os.Remove(staged.Name())

			return "", fmt.Errorf("the executable in the archive exceeds %d bytes", maxUpdateBinary)
		}
		if written == 0 {
			os.Remove(staged.Name())

			return "", fmt.Errorf("the executable in the archive is empty")
		}

		return staged.Name(), nil
	}

	return "", fmt.Errorf("the archive has no %s executable", binaryName)
}

func updateExtractZip(archivePath, dir, binaryName string) (string, error) {
	var reader *zip.ReadCloser
	var entry *zip.File
	var source io.ReadCloser
	var staged *os.File
	var written int64

	var err error

	reader, err = zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("the archive is not zip: %w", err)
	}
	defer reader.Close()

	for _, entry = range reader.File {
		if filepath.Base(entry.Name) != binaryName {
			continue
		}

		source, err = entry.Open()
		if err != nil {
			return "", err
		}

		staged, err = os.CreateTemp(dir, updateBinaryName+".new")
		if err != nil {
			source.Close()

			return "", fmt.Errorf("cannot stage the executable in %s: %w", dir, err)
		}

		written, err = io.Copy(staged, io.LimitReader(source, maxUpdateBinary+1))
		source.Close()
		staged.Close()
		if err != nil {
			os.Remove(staged.Name())

			return "", err
		}
		if written > maxUpdateBinary {
			os.Remove(staged.Name())

			return "", fmt.Errorf("the executable in the archive exceeds %d bytes", maxUpdateBinary)
		}
		if written == 0 {
			os.Remove(staged.Name())

			return "", fmt.Errorf("the executable in the archive is empty")
		}

		return staged.Name(), nil
	}

	return "", fmt.Errorf("the archive has no %s executable", binaryName)
}

func updateExtract(archivePath, dir string) (string, error) {
	if updateAssetExt() == ".zip" {
		return updateExtractZip(archivePath, dir, updateBinaryFile())
	}

	return updateExtractTarGz(archivePath, dir, updateBinaryFile())
}

func updateStage(ctx context.Context, url, want, dir string) (string, error) {
	var archivePath string
	var staged string

	var err error

	archivePath, err = updateDownloadArchive(ctx, url, want, dir)
	if err != nil {
		return "", err
	}
	defer os.Remove(archivePath)

	staged, err = updateExtract(archivePath, dir)
	if err != nil {
		return "", err
	}

	return staged, nil
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

func updateReplaceUnix(staged, target string) error {
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

func updateReplaceWindows(staged, target string) error {
	var previous string

	var err error

	previous = target + ".old"

	os.Remove(previous)

	err = os.Rename(target, previous)
	if err != nil {
		os.Remove(staged)

		return fmt.Errorf("moving the running executable aside failed: %w", err)
	}

	err = os.Rename(staged, target)
	if err != nil {
		os.Rename(previous, target)

		return fmt.Errorf("replacing %s failed: %w", target, err)
	}

	os.Remove(previous)

	return nil
}

func updateReplace(staged, target string) error {
	if runtime.GOOS == "windows" {
		return updateReplaceWindows(staged, target)
	}

	return updateReplaceUnix(staged, target)
}

func updateCheckSkipped(cmd *cobra.Command) bool {
	var current *cobra.Command

	for current = cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "update", "serve":
			return true
		}
	}

	return false
}

func updateCheckStart(cmd *cobra.Command) {
	var cached util.UpdateCache

	if util.AppVersion == "dev" || os.Getenv("MININARU_NO_UPDATE_CHECK") != "" || updateCheckSkipped(cmd) {
		return
	}

	cached = util.UpdateCacheRead()
	if time.Since(time.Unix(cached.CheckedAt, 0)) < util.UpdateCacheTTL {
		return
	}

	go func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var latest *release

		var err error

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		latest, err = updateLatestRelease(ctx)
		if err != nil {
			util.Log.Debug("the background update check failed", "error", err)

			return
		}

		util.UpdateCacheWrite(latest.TagName)
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

	var err error

	latest, err = updateFetchRelease(cmd.Context(), updateTagRef)
	if err != nil {
		return err
	}

	if updateTagRef == "" {
		util.UpdateCacheWrite(latest.TagName)
	}

	if updateCheckRef {
		fmt.Printf("installed: %s\n", util.AppVersion)
		fmt.Printf("latest:    %s\n", latest.TagName)

		if latest.TagName == util.AppVersion {
			fmt.Println("already up to date")

			return nil
		}

		fmt.Println("run `mininaru update` to install it")

		return nil
	}

	if latest.TagName == util.AppVersion && !updateForceRef {
		fmt.Printf("already running %s\n", util.AppVersion)
		fmt.Println("pass --force to reinstall it")

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

	fmt.Printf("downloading %s\n", assetName)

	staged, err = updateStage(cmd.Context(), assetUrl, want, filepath.Dir(target))
	if err != nil {
		return err
	}

	err = updateReplace(staged, target)
	if err != nil {
		return err
	}

	fmt.Printf("updated %s to %s\n", target, latest.TagName)

	return nil
}

func init() {
	updateCmd.Flags().StringVar(&updateTagRef, "tag", "", "release tag to install, defaults to the latest release")
	updateCmd.Flags().BoolVar(&updateCheckRef, "check", false, "report the installed and latest versions without installing")
	updateCmd.Flags().BoolVar(&updateForceRef, "force", false, "reinstall even when the latest version is already running")
}
