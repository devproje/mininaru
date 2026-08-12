// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"io"
	"log"
	"strings"
	"testing"
)

func TestPublicFailureDoesNotExposeCause(t *testing.T) {
	var originalWriter io.Writer
	var message string

	originalWriter = log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalWriter)

	message = publicFailure("request", errors.New("private upstream detail"))
	if strings.Contains(message, "private upstream detail") {
		t.Fatalf("public failure leaked cause: %q", message)
	}
	if !strings.Contains(message, "Please try again") {
		t.Fatalf("public failure has no recovery action: %q", message)
	}
}

func TestConversationFailureOffersFreshStart(t *testing.T) {
	var originalWriter io.Writer
	var message string

	originalWriter = log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalWriter)

	message = conversationFailure("answering", errors.New("private upstream detail"))
	if !strings.Contains(message, "`/reset`") {
		t.Fatalf("conversation failure has no fresh-start action: %q", message)
	}
	if strings.Contains(message, "private upstream detail") {
		t.Fatalf("conversation failure leaked cause: %q", message)
	}
}

func TestAccessDeniedExplainsNextStep(t *testing.T) {
	var message string

	message = accessDenied(true)
	if !strings.Contains(message, "`/user add`") {
		t.Fatalf("paired bot denial has no admin action: %q", message)
	}

	message = accessDenied(false)
	if !strings.Contains(message, "`mininaru bot pair`") || !strings.Contains(message, "`/pair`") {
		t.Fatalf("unpaired bot denial has no pairing action: %q", message)
	}
}
