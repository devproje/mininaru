// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"context"
	"net"
	"sync"
	"time"

	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type pairingRate struct {
	Started time.Time
	Count   int
}

type pairingService struct {
	mininaruv1.UnimplementedPairingServiceServer

	identity *ServerIdentity
	rates    map[string]pairingRate
	mu       sync.Mutex
}

const pairingPollInterval = 250 * time.Millisecond

const pairingRateWindow = time.Minute

const pairingRateLimit = 5

func pairingState(value string) mininaruv1.PairingState {
	switch value {
	case pairingWaiting:
		return mininaruv1.PairingState_PAIRING_STATE_WAITING
	case pairingApproved:
		return mininaruv1.PairingState_PAIRING_STATE_APPROVED
	case pairingDenied:
		return mininaruv1.PairingState_PAIRING_STATE_DENIED
	default:
		return mininaruv1.PairingState_PAIRING_STATE_EXPIRED
	}
}

func pairingPeer(ctx context.Context) string {
	var remote *peer.Peer
	var found bool
	var host string

	var err error

	remote, found = peer.FromContext(ctx)
	if !found || remote.Addr == nil {
		return "unknown"
	}

	host, _, err = net.SplitHostPort(remote.Addr.String())
	if err != nil {
		return remote.Addr.String()
	}

	return host
}

func (s *pairingService) allowPairing(ctx context.Context) bool {
	var key string
	var current pairingRate
	var now time.Time

	key = pairingPeer(ctx)
	now = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	current = s.rates[key]
	if current.Started.IsZero() || now.Sub(current.Started) >= pairingRateWindow {
		current = pairingRate{Started: now}
	}
	if current.Count >= pairingRateLimit {
		s.rates[key] = current
		return false
	}

	current.Count++
	s.rates[key] = current

	return true
}

func (s *pairingService) Begin(ctx context.Context, request *mininaruv1.BeginPairingRequest) (*mininaruv1.BeginPairingResponse, error) {
	var pairing *PairingRequest

	var err error

	if !s.allowPairing(ctx) {
		return nil, status.Error(codes.ResourceExhausted, "too many pairing requests")
	}

	pairing, err = PairingBegin(request.GetDeviceName(), request.GetPublicKey())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &mininaruv1.BeginPairingResponse{RequestId: pairing.Id, PairingCode: pairing.Code,
		ClientFingerprint: pairing.Fingerprint, ExpiresAtUnix: pairing.ExpiresAt}, nil
}

func pairingEvent(request *PairingRequest, ca []byte) *mininaruv1.PairingEvent {
	var event mininaruv1.PairingEvent

	event.State = pairingState(request.Status)
	if request.Status == pairingApproved {
		event.ClientCertificatePem = request.CertificatePEM
		event.CaCertificatePem = ca
	}

	return &event
}

func (s *pairingService) Watch(request *mininaruv1.WatchPairingRequest, stream mininaruv1.PairingService_WatchServer) error {
	var ticker *time.Ticker
	var pairing *PairingRequest
	var last string
	var event *mininaruv1.PairingEvent

	var err error

	if request.GetRequestId() == "" {
		return status.Error(codes.InvalidArgument, "request id is required")
	}

	ticker = time.NewTicker(pairingPollInterval)
	defer ticker.Stop()

	for {
		pairing, err = PairingGet(request.GetRequestId())
		if err != nil {
			return status.Error(codes.NotFound, err.Error())
		}
		if pairing.Status != last {
			event = pairingEvent(pairing, s.identity.CAPEM)
			err = stream.Send(event)
			if err != nil {
				return err
			}
			last = pairing.Status
		}
		if pairing.Status != pairingWaiting {
			return nil
		}

		select {
		case <-ticker.C:
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
