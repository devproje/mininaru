// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const (
	DEFAULT_SERVER_HOST string = "127.0.0.1"
	DEFAULT_SERVER_PORT uint16 = 8223

	shutdownTimeout time.Duration = 5 * time.Second
)

var (
	hostRef string
	portRef uint16

	webapp *http.Server
)

func shutdown(ctx context.Context) {
	var shutdownCtx context.Context
	var cancel context.CancelFunc

	var err error

	<-ctx.Done()
	util.Log.Info("shutting down mininaru...")

	shutdownCtx, cancel = context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err = webapp.Shutdown(shutdownCtx)
	if err != nil {
		util.Log.Error("mininaru shutdown failed", "error", err)
	}
}

func execServe(cmd *cobra.Command, args []string) error {
	var app *core.NaruCore
	var ctx context.Context
	var stop context.CancelFunc

	var err error

	app = core.NewNaruCore()
	webapp = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", hostRef, portRef),
		Handler: app.App,
	}

	ctx, stop = signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go shutdown(ctx)

	util.Log.Info(fmt.Sprintf("mininaru bind at http://%s:%d", hostRef, portRef))
	err = webapp.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

var serve *cobra.Command = &cobra.Command{
	Use:  "serve",
	RunE: execServe,
}

func init() {
	serve.Flags().StringVar(&hostRef, "host", DEFAULT_SERVER_HOST, "mininaru daemon host")
	serve.Flags().Uint16Var(&portRef, "port", DEFAULT_SERVER_PORT, "mininaru daemon port")
}
