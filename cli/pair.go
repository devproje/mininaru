// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devproje/mininaru/config"
	mininarurpc "github.com/devproje/mininaru/rpc"
	"github.com/spf13/cobra"
)

var (
	pairNameRef        string
	pairFingerprintRef string
)

var pairCmd *cobra.Command = &cobra.Command{
	Use:   "pair <address>",
	Short: "pair this device with a mininaru grpc server",
	Long: `Verify the server fingerprint, create an Ed25519 device identity, and
wait for the server operator to approve the one-time pairing code.`,
	Example: `  mininaru pair naru.example.com:9090
  mininaru pair 127.0.0.1:9090 --name laptop`,
	Args: usageArgs(cobra.ExactArgs(1)),
	RunE: pairExecute,
}

func pairDeviceName() string {
	var name string

	if pairNameRef != "" {
		return pairNameRef
	}

	name, _ = os.Hostname()
	if name == "" {
		name = "mininaru-client"
	}

	return name
}

func pairTrust(fingerprint, expected string) (bool, error) {
	if expected != "" {
		if strings.TrimSpace(expected) != fingerprint {
			return false, fmt.Errorf("server fingerprint mismatch: got %s", fingerprint)
		}

		return true, nil
	}

	fmt.Fprintf(askOut, "Server fingerprint:\n%s\n\n", fingerprint)

	return askConfirm("Trust this server", false)
}

func pairWithServer(ctx context.Context, address, name, expected string) error {
	var fingerprint string
	var trusted bool

	var err error

	address = strings.TrimSpace(address)
	fingerprint, err = mininarurpc.ServerFingerprint(ctx, address)
	if err != nil {
		return err
	}

	trusted, err = pairTrust(fingerprint, expected)
	if err != nil {
		return err
	}
	if !trusted {
		return fmt.Errorf("server was not trusted")
	}

	err = mininarurpc.Pair(ctx, address, name, fingerprint, func(request *mininarurpc.PairingRequest) {
		uiNote("pairing code: %s", request.Code)
		uiNote("client fingerprint: %s", request.Fingerprint)
		uiNote("waiting for approval on the server")
	})
	if err != nil {
		return err
	}

	config.Client.Server.Address = address
	config.Client.Mode = config.ModeClient
	err = config.ClientSave()
	if err != nil {
		return err
	}

	uiOk("paired with %s", address)

	return nil
}

func pairExecute(cmd *cobra.Command, args []string) error {
	return pairWithServer(cmd.Context(), args[0], pairDeviceName(), pairFingerprintRef)
}

func init() {
	pairCmd.Flags().StringVar(&pairNameRef, "name", "", "device name shown to the server operator")
	pairCmd.Flags().StringVar(&pairFingerprintRef, "fingerprint", "", "expected server fingerprint for non-interactive pairing")
}
