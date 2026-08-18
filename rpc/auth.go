// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"context"
	"crypto/x509"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const pairingMethodPrefix = "/mininaru.v1.PairingService/"

func clientCertificate(ctx context.Context) (*x509.Certificate, error) {
	var remote *peer.Peer
	var found bool
	var info credentials.TLSInfo
	var ok bool

	remote, found = peer.FromContext(ctx)
	if !found {
		return nil, status.Error(codes.Unauthenticated, "client certificate is required")
	}

	info, ok = remote.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.PeerCertificates) == 0 || len(info.State.VerifiedChains) == 0 {
		return nil, status.Error(codes.Unauthenticated, "valid client certificate is required")
	}

	return info.State.PeerCertificates[0], nil
}

func authenticate(ctx context.Context) error {
	var certificate *x509.Certificate

	var err error

	certificate, err = clientCertificate(ctx)
	if err != nil {
		return err
	}

	_, err = ClientAuthenticate(certificate)
	if err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	return nil
}

func unaryAuthenticate(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	var err error

	if strings.HasPrefix(info.FullMethod, pairingMethodPrefix) {
		return handler(ctx, request)
	}

	err = authenticate(ctx)
	if err != nil {
		return nil, err
	}

	return handler(ctx, request)
}

func streamAuthenticate(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	var err error

	if strings.HasPrefix(info.FullMethod, pairingMethodPrefix) {
		return handler(server, stream)
	}

	err = authenticate(stream.Context())
	if err != nil {
		return err
	}

	return handler(server, stream)
}
