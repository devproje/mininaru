// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/devproje/mininaru/server"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const (
	SERVER_DEFAULT_HOST string = "0.0.0.0"
	SERVER_DEFAULT_PORT uint16 = 8223
)

var serve *cobra.Command = &cobra.Command{
	Use:   "serve",
	Short: "serve HTTP server",
	Long:  "Running HTTP endpoint server for mininaru",

	Example: fmt.Sprintf("  mininaru serve\n  mininaru serve --host %s --port %d", SERVER_DEFAULT_HOST, SERVER_DEFAULT_PORT),
	RunE:    serveExecute,
}

var (
	serverHostRef string
	serverPortRef uint16
)

func init() {
	serve.Flags().StringVar(&serverHostRef, "host", SERVER_DEFAULT_HOST, "address to bind the server")
	serve.Flags().Uint16Var(&serverPortRef, "port", SERVER_DEFAULT_PORT, "port to bind the server")

	serve.Flags().BoolVar(&util.AppDebug, "debug", false, "running mininaru debug mode")
}

func serveExecute(cmd *cobra.Command, args []string) error {
	var err error

	server.App = server.NewAppServer(serverHostRef, serverPortRef)

	fmt.Printf("webserver bind at http://%s:%d\n", serverHostRef, serverPortRef)
	err = server.App.WebServer.ListenAndServe()
	if err != nil {
		return err
	}

	return nil
}
