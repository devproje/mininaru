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
}
