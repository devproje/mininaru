// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
)

func TestExitCodeByErrorKind(t *testing.T) {
	var cases []struct {
		err  error
		code int
	}
	var current struct {
		err  error
		code int
	}

	cases = []struct {
		err  error
		code int
	}{
		{errors.New("boom"), exitFailure},
		{usageErrorf("bad flag"), exitUsage},
		{configErrorf("nothing configured"), exitConfig},
		{fmt.Errorf("wrapped: %w", configErrorf("nothing configured")), exitConfig},
	}

	for _, current = range cases {
		if exitCode(current.err) != current.code {
			t.Fatalf("exit code for %q was %d, expected %d", current.err, exitCode(current.err), current.code)
		}
	}
}

func TestUsageArgsWrapsValidationFailures(t *testing.T) {
	var check cobra.PositionalArgs

	var err error

	check = usageArgs(cobra.NoArgs)

	err = check(&cobra.Command{Use: "demo"}, []string{"extra"})
	if err == nil {
		t.Fatal("unexpected argument was accepted")
	}

	if exitCode(err) != exitUsage {
		t.Fatalf("argument error mapped to exit code %d, expected %d", exitCode(err), exitUsage)
	}

	err = check(&cobra.Command{Use: "demo"}, nil)
	if err != nil {
		t.Fatalf("valid arguments were rejected: %v", err)
	}
}
