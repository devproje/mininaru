// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package attachments

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

const maxAttachments = 4
const maxAttachmentBytes = 10 * 1024 * 1024
const maxTotalBytes = 20 * 1024 * 1024

var client = &http.Client{Timeout: 20 * time.Second}

func allowedURL(raw string) bool {
	var parsed *url.URL
	var host string

	var err error

	parsed, err = url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return false
	}
	host = strings.ToLower(parsed.Hostname())
	return host == "cdn.discordapp.com" || host == "media.discordapp.net"
}

func textType(contentType, filename string) bool {
	var mediaType string
	var ext string

	mediaType, _, _ = mime.ParseMediaType(contentType)
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json", "application/xml", "application/yaml", "application/x-yaml", "application/javascript":
		return true
	}
	ext = strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".rs", ".java", ".c", ".h", ".cpp", ".hpp",
		".sh", ".sql", ".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".xml", ".csv", ".log":
		return true
	}
	return false
}

func imageType(contentType string) bool {
	var mediaType string

	mediaType, _, _ = mime.ParseMediaType(contentType)
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	}
	return false
}

func download(ctx context.Context, attachment *discordgo.MessageAttachment) ([]byte, error) {
	var request *http.Request
	var response *http.Response
	var reader io.Reader
	var buf []byte

	var err error

	if attachment == nil || !allowedURL(attachment.URL) {
		return nil, fmt.Errorf("attachment URL is not a Discord CDN URL")
	}
	if attachment.Size > maxAttachmentBytes {
		return nil, fmt.Errorf("attachment %q exceeds the 10 MiB limit", attachment.Filename)
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, attachment.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %q returned %s", attachment.Filename, response.Status)
	}
	reader = io.LimitReader(response.Body, maxAttachmentBytes+1)
	buf, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(buf) > maxAttachmentBytes {
		return nil, fmt.Errorf("attachment %q exceeds the 10 MiB limit", attachment.Filename)
	}
	return buf, nil
}

func Build(ctx context.Context, content string, files []*discordgo.MessageAttachment) ([]openai.ChatCompletionContentPartUnionParam, error) {
	var parts []openai.ChatCompletionContentPartUnionParam
	var attachment *discordgo.MessageAttachment
	var buf []byte
	var total int
	var mediaType string
	var detectedType string
	var encoded string

	var err error

	if len(files) > maxAttachments {
		return nil, fmt.Errorf("at most %d attachments are supported", maxAttachments)
	}
	if strings.TrimSpace(content) == "" {
		content = "Analyze the attached content."
	}
	parts = append(parts, openai.TextContentPart(content))
	for _, attachment = range files {
		buf, err = download(ctx, attachment)
		if err != nil {
			return nil, err
		}
		total += len(buf)
		if total > maxTotalBytes {
			return nil, fmt.Errorf("attachments exceed the 20 MiB total limit")
		}
		mediaType, _, _ = mime.ParseMediaType(attachment.ContentType)
		detectedType, _, _ = mime.ParseMediaType(http.DetectContentType(buf))
		if imageType(mediaType) {
			if !imageType(detectedType) {
				return nil, fmt.Errorf("attachment %q content does not match type %q", attachment.Filename, mediaType)
			}
			mediaType = detectedType
		}
		if mediaType == "" || mediaType == "application/octet-stream" {
			mediaType = detectedType
		}
		encoded = base64.StdEncoding.EncodeToString(buf)
		switch {
		case imageType(mediaType):
			parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL: "data:" + mediaType + ";base64," + encoded, Detail: "auto",
			}))
		case textType(mediaType, attachment.Filename):
			parts = append(parts, openai.TextContentPart(fmt.Sprintf("Attachment %q:\n%s", attachment.Filename, string(buf))))
		case mediaType == "application/pdf":
			parts = append(parts, openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
				FileData: param.NewOpt(encoded), Filename: param.NewOpt(attachment.Filename),
			}))
		default:
			return nil, fmt.Errorf("attachment %q has unsupported type %q", attachment.Filename, mediaType)
		}
	}
	return parts, nil
}
