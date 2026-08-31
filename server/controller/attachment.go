// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"database/sql"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const attachmentMaxBytes int64 = 20 << 20

func sessionExists(ctx *gin.Context, id string) bool {
	var err error

	_, err = core.SessionRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return false
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}

	return true
}

func sniffImageMime(path string) (string, error) {
	var file *os.File
	var head []byte
	var n int
	var mime string

	var err error

	file, err = os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	head = make([]byte, 512)
	n, _ = file.Read(head)
	mime = http.DetectContentType(head[:n])

	if !strings.HasPrefix(mime, "image/") {
		return "", errors.New("only image attachments are supported")
	}

	return mime, nil
}

func AttachmentUpload(ctx *gin.Context) {
	var sessionId string
	var header *multipart.FileHeader
	var id string
	var dir string
	var dest string
	var mime string

	var err error

	sessionId = ctx.Param("id")
	if !sessionExists(ctx, sessionId) {
		return
	}

	header, err = ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "a multipart 'file' field is required"})
		return
	}

	if header.Size > attachmentMaxBytes {
		ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "attachment is too large"})
		return
	}

	id = uuid.NewString()
	dir = util.Path("attachments")

	err = os.MkdirAll(dir, 0700)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dest = filepath.Join(dir, id)

	err = ctx.SaveUploadedFile(header, dest)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mime, err = sniffImageMime(dest)
	if err != nil {
		os.Remove(dest)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	os.Chmod(dest, 0600)

	err = core.AttachmentCreate(&core.Attachment{Id: id, SessionId: sessionId, Mime: mime, Bytes: header.Size, Path: dest})
	if err != nil {
		os.Remove(dest)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"id": id, "mime": mime, "bytes": header.Size})
}

func AttachmentDownload(ctx *gin.Context) {
	var att *core.Attachment

	var err error

	att, err = core.AttachmentRead(ctx.Param("id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "attachment not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-Type", att.Mime)
	ctx.File(att.Path)
}
