package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	exitFailure = 1
	exitUsage   = 2
	exitConfig  = 3
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func usageErrorf(format string, args ...any) error {
	return &exitError{code: exitUsage, err: fmt.Errorf(format, args...)}
}

func configErrorf(format string, args ...any) error {
	return &exitError{code: exitConfig, err: fmt.Errorf(format, args...)}
}

func usageArgs(check cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		var err error

		err = check(cmd, args)
		if err != nil {
			return usageErrorf("%v", err)
		}

		return nil
	}
}

func usageFlagError(cmd *cobra.Command, err error) error {
	return usageErrorf("%v", err)
}

func exitCode(err error) int {
	var typed *exitError

	if errors.As(err, &typed) {
		return typed.code
	}

	return exitFailure
}

func reportError(err error) {
	fmt.Fprintln(os.Stderr, errStyle.Render("error: ")+err.Error())
}
