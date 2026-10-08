// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import "github.com/gin-gonic/gin"

func errorTemplate(message string) gin.H {
	return gin.H{
		"ok":    0,
		"error": message,
	}
}

var (
	errInvalidPayload        = errorTemplate("invalid request body")
	errProviderCreate        = errorTemplate("failed to create provider")
	errProviderList          = errorTemplate("failed to list providers")
	errProviderRead          = errorTemplate("failed to read provider")
	errProviderNotFound      = errorTemplate("provider not found")
	errProviderUpdate        = errorTemplate("failed to update provider")
	errProviderDelete        = errorTemplate("failed to delete provider")
	errInvalidProfileModel   = errorTemplate("model must use provider:model with a registered provider")
	errProfileSave           = errorTemplate("failed to save profile")
	errProfileRead           = errorTemplate("failed to read profile")
	errProfileNotInitialized = errorTemplate("profile is not initialized")
	errModelList             = errorTemplate("failed to list available models")
	errModelValidation       = errorTemplate("failed to verify model availability")
)
