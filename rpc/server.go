// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/devproje/mininaru/core"
	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
	"github.com/devproje/mininaru/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Host string
	Port int
}

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 9090
)

const (
	maxReceiveMessageBytes = 20 << 20
	maxSendMessageBytes    = 20 << 20
	gracefulStopTimeout    = 5 * time.Second
)

func tlsConfig(identity *ServerIdentity) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{identity.Certificate},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    identity.CAPool,
		MinVersion:   tls.VersionTLS13,
	}
}

func NewServer(identity *ServerIdentity, registry *core.Registry) (*grpc.Server, error) {
	var server *grpc.Server

	if identity == nil {
		return nil, fmt.Errorf("server identity is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is required")
	}

	server = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig(identity))),
		grpc.UnaryInterceptor(unaryAuthenticate),
		grpc.StreamInterceptor(streamAuthenticate),
		grpc.MaxRecvMsgSize(maxReceiveMessageBytes),
		grpc.MaxSendMsgSize(maxSendMessageBytes),
	)
	mininaruv1.RegisterPairingServiceServer(server, &pairingService{identity: identity, rates: make(map[string]pairingRate)})
	mininaruv1.RegisterMininaruServiceServer(server, &mininaruService{registry: registry, slots: make(chan struct{}, maxConcurrentChats)})

	return server, nil
}

func gracefulStop(server *grpc.Server) {
	var stopped chan struct{}
	var timer *time.Timer

	stopped = make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	timer = time.NewTimer(gracefulStopTimeout)
	defer timer.Stop()

	select {
	case <-stopped:
	case <-timer.C:
		server.Stop()
	}
}

func Serve(ctx context.Context, cfg Config, registry *core.Registry) error {
	var identity *ServerIdentity
	var address string
	var listener net.Listener
	var server *grpc.Server
	var errs chan error

	var err error

	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}

	identity, err = LoadServerIdentity()
	if err != nil {
		return err
	}

	address = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	listener, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	server, err = NewServer(identity, registry)
	if err != nil {
		return err
	}

	errs = make(chan error, 1)
	go func() {
		errs <- server.Serve(listener)
	}()

	util.Log.Info("grpc server listening", "address", listener.Addr().String(), "fingerprint", identity.Fingerprint)

	select {
	case err = <-errs:
		return err
	case <-ctx.Done():
	}

	gracefulStop(server)
	util.Log.Info("grpc server stopped")

	return nil
}
