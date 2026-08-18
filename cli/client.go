// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"time"

	mininarurpc "github.com/devproje/mininaru/rpc"
	"github.com/spf13/cobra"
)

var clientConfig *cobra.Command = &cobra.Command{
	Use:   "client",
	Short: "manage gRPC client devices and pairing requests",
	Args:  usageArgs(cobra.NoArgs),
}

var clientList *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list paired gRPC client devices",
	Args:  usageArgs(cobra.NoArgs),
	RunE:  clientListExecute,
}

var clientPending *cobra.Command = &cobra.Command{
	Use:   "pending",
	Short: "list waiting gRPC pairing requests",
	Args:  usageArgs(cobra.NoArgs),
	RunE:  clientPendingExecute,
}

var clientApprove *cobra.Command = &cobra.Command{
	Use:   "approve <code>",
	Short: "approve a waiting gRPC pairing request",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  clientApproveExecute,
}

var clientDeny *cobra.Command = &cobra.Command{
	Use:   "deny <code>",
	Short: "deny a waiting gRPC pairing request",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  clientDenyExecute,
}

var clientRevoke *cobra.Command = &cobra.Command{
	Use:   "revoke <id-or-fingerprint>",
	Short: "revoke a paired gRPC client device",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  clientRevokeExecute,
}

func clientState(device *mininarurpc.ClientDevice) string {
	if device.RevokedAt != 0 {
		return "revoked"
	}

	return "active"
}

func clientSeen(value int64) string {
	if value == 0 {
		return "never"
	}

	return time.Unix(value, 0).Format("2006-01-02 15:04")
}

func clientListExecute(cmd *cobra.Command, args []string) error {
	var devices []*mininarurpc.ClientDevice
	var device *mininarurpc.ClientDevice
	var rows *uiRows

	var err error

	devices, err = mininarurpc.ClientList()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		uiEmpty("no grpc clients paired")
		return nil
	}

	rows = uiTable("NAME", "FINGERPRINT", "STATE", "LAST SEEN")
	for _, device = range devices {
		rows.row(device.Name, device.Fingerprint, clientState(device), clientSeen(device.LastSeenAt))
	}
	rows.flush()

	return nil
}

func clientPendingExecute(cmd *cobra.Command, args []string) error {
	var requests []*mininarurpc.PairingRequest
	var request *mininarurpc.PairingRequest
	var rows *uiRows

	var err error

	requests, err = mininarurpc.PairingPending()
	if err != nil {
		return err
	}
	if len(requests) == 0 {
		uiEmpty("no grpc pairing requests waiting")
		return nil
	}

	rows = uiTable("CODE", "DEVICE", "FINGERPRINT", "EXPIRES")
	for _, request = range requests {
		rows.row(request.Code, request.Name, request.Fingerprint, clientSeen(request.ExpiresAt))
	}
	rows.flush()

	return nil
}

func clientApproveExecute(cmd *cobra.Command, args []string) error {
	var device *mininarurpc.ClientDevice

	var err error

	device, err = mininarurpc.PairingApprove(args[0])
	if err != nil {
		return err
	}

	uiOk("paired %s (%s)", device.Name, device.Fingerprint)

	return nil
}

func clientDenyExecute(cmd *cobra.Command, args []string) error {
	var err error

	err = mininarurpc.PairingDeny(args[0])
	if err != nil {
		return err
	}

	uiOk("denied pairing request %s", args[0])

	return nil
}

func clientRevokeExecute(cmd *cobra.Command, args []string) error {
	var err error

	err = mininarurpc.ClientRevoke(args[0])
	if err != nil {
		return err
	}

	uiOk("revoked grpc client %s", args[0])

	return nil
}

func init() {
	clientConfig.AddCommand(clientList)
	clientConfig.AddCommand(clientPending)
	clientConfig.AddCommand(clientApprove)
	clientConfig.AddCommand(clientDeny)
	clientConfig.AddCommand(clientRevoke)
}
