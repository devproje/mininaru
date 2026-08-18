// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
	"github.com/devproje/mininaru/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type KnownServer struct {
	Address     string `json:"address"`
	Fingerprint string `json:"fingerprint"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
	CA          string `json:"ca"`
}

type KnownServers struct {
	Servers []*KnownServer `json:"servers"`
}

const knownServersFile = "servers.json"

const identityDirectory = "identity"

const dialTimeout = 10 * time.Second

func serverKey(address string) string {
	var sum [sha256.Size]byte

	sum = sha256.Sum256([]byte(address))

	return hex.EncodeToString(sum[:8])
}

func loadKnownServers() (*KnownServers, error) {
	var servers KnownServers
	var bytes []byte

	var err error

	bytes, err = os.ReadFile(util.Path(knownServersFile))
	if err != nil {
		if os.IsNotExist(err) {
			return &servers, nil
		}

		return nil, err
	}

	err = json.Unmarshal(bytes, &servers)
	if err != nil {
		return nil, err
	}

	return &servers, nil
}

func knownServer(address string) (*KnownServer, error) {
	var servers *KnownServers
	var server *KnownServer

	var err error

	servers, err = loadKnownServers()
	if err != nil {
		return nil, err
	}

	for _, server = range servers.Servers {
		if server.Address == address {
			return server, nil
		}
	}

	return nil, fmt.Errorf("server %s is not paired", address)
}

func saveKnownServer(server *KnownServer) error {
	var servers *KnownServers
	var current *KnownServer
	var found bool
	var bytes []byte

	var err error

	servers, err = loadKnownServers()
	if err != nil {
		return err
	}

	for _, current = range servers.Servers {
		if current.Address != server.Address {
			continue
		}

		*current = *server
		found = true
		break
	}
	if !found {
		servers.Servers = append(servers.Servers, server)
	}

	bytes, err = json.MarshalIndent(servers, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(util.Path(knownServersFile), bytes, 0600)
}

func peerFingerprint(state tls.ConnectionState) (string, error) {
	if len(state.PeerCertificates) == 0 {
		return "", fmt.Errorf("server sent no certificate")
	}

	return certificateFingerprint(state.PeerCertificates[0])
}

func fingerprintTLS(expected string) *tls.Config {
	var config tls.Config

	config.MinVersion = tls.VersionTLS13
	config.InsecureSkipVerify = true
	config.VerifyConnection = func(state tls.ConnectionState) error {
		var fingerprint string

		var err error

		fingerprint, err = peerFingerprint(state)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(fingerprint), []byte(expected)) != 1 {
			return fmt.Errorf("server fingerprint changed: got %s", fingerprint)
		}
		if time.Now().Before(state.PeerCertificates[0].NotBefore) || time.Now().After(state.PeerCertificates[0].NotAfter) {
			return fmt.Errorf("server certificate is not currently valid")
		}

		return nil
	}

	return &config
}

func ServerFingerprint(ctx context.Context, address string) (string, error) {
	var dialer net.Dialer
	var config tls.Config
	var tlsDialer tls.Dialer
	var raw net.Conn
	var connection *tls.Conn
	var state tls.ConnectionState
	var ok bool

	var err error

	config.MinVersion = tls.VersionTLS13
	config.InsecureSkipVerify = true
	dialer.Timeout = dialTimeout
	tlsDialer.NetDialer = &dialer
	tlsDialer.Config = &config

	raw, err = tlsDialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	connection, ok = raw.(*tls.Conn)
	if !ok {
		raw.Close()
		return "", fmt.Errorf("server connection is not tls")
	}
	defer connection.Close()

	state = connection.ConnectionState()

	return peerFingerprint(state)
}

func pairingConnection(address, fingerprint string) (*grpc.ClientConn, error) {
	return grpc.NewClient(address,
		grpc.WithTransportCredentials(credentials.NewTLS(fingerprintTLS(fingerprint))),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxReceiveMessageBytes), grpc.MaxCallSendMsgSize(maxSendMessageBytes)))
}

func encodePrivateKey(privateKey ed25519.PrivateKey) ([]byte, error) {
	var encoded []byte

	var err error

	encoded, err = x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), nil
}

func saveClientIdentity(address, fingerprint string, privateKey ed25519.PrivateKey, certificate, ca []byte) error {
	var directory string
	var key string
	var certificatePath string
	var privateKeyPath string
	var caPath string
	var privateKeyPEM []byte
	var server KnownServer

	var err error

	directory = util.Path(identityDirectory)
	err = os.MkdirAll(directory, 0700)
	if err != nil {
		return err
	}

	key = serverKey(address)
	certificatePath = filepath.Join(directory, key+".crt")
	privateKeyPath = filepath.Join(directory, key+".key")
	caPath = filepath.Join(directory, key+"-ca.crt")

	privateKeyPEM, err = encodePrivateKey(privateKey)
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(certificatePath, certificate, 0600)
	if err != nil {
		return err
	}
	err = util.WriteFileAtomic(privateKeyPath, privateKeyPEM, 0600)
	if err != nil {
		return err
	}
	err = util.WriteFileAtomic(caPath, ca, 0600)
	if err != nil {
		return err
	}

	server = KnownServer{Address: address, Fingerprint: fingerprint,
		Certificate: certificatePath, PrivateKey: privateKeyPath, CA: caPath}

	return saveKnownServer(&server)
}

func Pair(ctx context.Context, address, deviceName, fingerprint string,
	onBegin func(*PairingRequest)) error {
	var publicKey ed25519.PublicKey
	var privateKey ed25519.PrivateKey
	var encodedPublicKey []byte
	var connection *grpc.ClientConn
	var client mininaruv1.PairingServiceClient
	var response *mininaruv1.BeginPairingResponse
	var request PairingRequest
	var watch mininaruv1.PairingService_WatchClient
	var event *mininaruv1.PairingEvent

	var err error

	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	encodedPublicKey, err = x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}

	connection, err = pairingConnection(address, fingerprint)
	if err != nil {
		return err
	}
	defer connection.Close()

	client = mininaruv1.NewPairingServiceClient(connection)
	response, err = client.Begin(ctx, &mininaruv1.BeginPairingRequest{PublicKey: encodedPublicKey, DeviceName: deviceName})
	if err != nil {
		return err
	}

	request = PairingRequest{Id: response.GetRequestId(), Code: response.GetPairingCode(), Name: deviceName,
		Fingerprint: response.GetClientFingerprint(), ExpiresAt: response.GetExpiresAtUnix(), Status: pairingWaiting}
	if onBegin != nil {
		onBegin(&request)
	}

	watch, err = client.Watch(ctx, &mininaruv1.WatchPairingRequest{RequestId: response.GetRequestId()})
	if err != nil {
		return err
	}

	for {
		event, err = watch.Recv()
		if err != nil {
			return err
		}
		switch event.GetState() {
		case mininaruv1.PairingState_PAIRING_STATE_WAITING:
			continue
		case mininaruv1.PairingState_PAIRING_STATE_APPROVED:
			return saveClientIdentity(address, fingerprint, privateKey,
				event.GetClientCertificatePem(), event.GetCaCertificatePem())
		case mininaruv1.PairingState_PAIRING_STATE_DENIED:
			return fmt.Errorf("pairing request was denied")
		default:
			return fmt.Errorf("pairing request expired")
		}
	}
}

func verifyKnownServer(server *KnownServer, roots *x509.CertPool) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		var fingerprint string
		var options x509.VerifyOptions

		var err error

		fingerprint, err = peerFingerprint(state)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(fingerprint), []byte(server.Fingerprint)) != 1 {
			return fmt.Errorf("server fingerprint changed: got %s", fingerprint)
		}

		options = x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		_, err = state.PeerCertificates[0].Verify(options)

		return err
	}
}

func Dial(ctx context.Context, address string) (*grpc.ClientConn, error) {
	var server *KnownServer
	var certificate tls.Certificate
	var ca []byte
	var roots *x509.CertPool
	var config tls.Config

	var err error

	address = strings.TrimSpace(address)
	server, err = knownServer(address)
	if err != nil {
		return nil, err
	}

	certificate, err = tls.LoadX509KeyPair(server.Certificate, server.PrivateKey)
	if err != nil {
		return nil, err
	}
	ca, err = os.ReadFile(server.CA)
	if err != nil {
		return nil, err
	}

	roots = x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("load paired server ca")
	}

	config = tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, VerifyConnection: verifyKnownServer(server, roots)}

	return grpc.NewClient(address,
		grpc.WithTransportCredentials(credentials.NewTLS(&config)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxReceiveMessageBytes), grpc.MaxCallSendMsgSize(maxSendMessageBytes)))
}
