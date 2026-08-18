// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
	"github.com/devproje/mininaru/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testChatStream struct {
	ctx      context.Context
	incoming chan *mininaruv1.ChatClientEvent
	outgoing []*mininaruv1.ChatServerEvent
	autoDeny bool
	mu       sync.Mutex
}

func (s *testChatStream) Send(event *mininaruv1.ChatServerEvent) error {
	s.mu.Lock()
	s.outgoing = append(s.outgoing, event)
	s.mu.Unlock()
	if s.autoDeny && event.GetApproval() != nil {
		s.incoming <- &mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Approval{Approval: &mininaruv1.ApprovalDecision{
			RequestId: event.GetApproval().GetRequestId(), Choice: mininaruv1.ApprovalChoice_APPROVAL_CHOICE_DENY}}}
	}

	return nil
}

func (s *testChatStream) Recv() (*mininaruv1.ChatClientEvent, error) {
	var event *mininaruv1.ChatClientEvent

	select {
	case event = <-s.incoming:
		return event, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *testChatStream) SetHeader(headers metadata.MD) error {
	return nil
}

func (s *testChatStream) SendHeader(headers metadata.MD) error {
	return nil
}

func (s *testChatStream) SetTrailer(trailers metadata.MD) {}

func (s *testChatStream) Context() context.Context {
	return s.ctx
}

func (s *testChatStream) SendMsg(message any) error {
	return nil
}

func (s *testChatStream) RecvMsg(message any) error {
	return io.EOF
}

func rpcTestSetup(t *testing.T) {
	var directory string

	var err error

	t.Helper()

	directory = t.TempDir()
	err = util.InitFS(directory)
	if err != nil {
		t.Fatal(err)
	}
	util.DB, err = util.InitDatabase(filepath.Join(directory, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		util.DB.Close()
	})
}

func testPublicKey(t *testing.T) (ed25519.PrivateKey, []byte) {
	var publicKey ed25519.PublicKey
	var privateKey ed25519.PrivateKey
	var encoded []byte

	var err error

	t.Helper()

	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}

	return privateKey, encoded
}

func testClientCertificate(t *testing.T, certificatePEM []byte, privateKey ed25519.PrivateKey) tls.Certificate {
	var key []byte
	var keyPEM []byte
	var certificate tls.Certificate

	var err error

	t.Helper()

	key, err = x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	certificate, err = tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	return certificate
}

func TestServerIdentityStableAndPrivate(t *testing.T) {
	var first *ServerIdentity
	var second *ServerIdentity
	var info os.FileInfo

	var err error

	rpcTestSetup(t)

	first, err = LoadServerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	second, err = LoadServerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprint changed: %s != %s", first.Fingerprint, second.Fingerprint)
	}

	info, err = os.Stat(filepath.Join(util.Path(pkiDirectory), caPrivateKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("ca key mode = %o, want 600", info.Mode().Perm())
	}
}

func TestPairingApprovesAuthenticatesAndRevokes(t *testing.T) {
	var privateKey ed25519.PrivateKey
	var publicKey []byte
	var request *PairingRequest
	var device *ClientDevice
	var approved *PairingRequest
	var block *pem.Block
	var certificate *x509.Certificate

	var err error

	rpcTestSetup(t)
	_, err = LoadServerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	privateKey, publicKey = testPublicKey(t)
	if len(privateKey) == 0 {
		t.Fatal("client private key is empty")
	}

	request, err = PairingBegin("laptop", publicKey)
	if err != nil {
		t.Fatal(err)
	}
	device, err = PairingApprove(request.Code)
	if err != nil {
		t.Fatal(err)
	}
	approved, err = PairingGet(request.Id)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != pairingApproved || len(approved.CertificatePEM) == 0 {
		t.Fatalf("approved pairing = %#v", approved)
	}

	block, _ = pem.Decode(approved.CertificatePEM)
	if block == nil {
		t.Fatal("client certificate is not pem")
	}
	certificate, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ClientAuthenticate(certificate)
	if err != nil {
		t.Fatal(err)
	}

	err = ClientRevoke(device.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ClientAuthenticate(certificate)
	if err == nil {
		t.Fatal("revoked client authenticated")
	}
}

func TestPairingBeginIsRateLimitedByPeer(t *testing.T) {
	var service pairingService
	var publicKey []byte
	var ctx context.Context
	var index int

	var err error

	rpcTestSetup(t)
	_, err = LoadServerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	_, publicKey = testPublicKey(t)
	service.rates = make(map[string]pairingRate)
	ctx = peer.NewContext(context.Background(), &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 1234}})

	for index = 0; index < pairingRateLimit; index++ {
		_, err = service.Begin(ctx, &mininaruv1.BeginPairingRequest{DeviceName: "client", PublicKey: publicKey})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = service.Begin(ctx, &mininaruv1.BeginPairingRequest{DeviceName: "client", PublicKey: publicKey})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("rate limit code = %v, want %v", status.Code(err), codes.ResourceExhausted)
	}
}

func TestGRPCRequiresActivePairedCertificate(t *testing.T) {
	var identity *ServerIdentity
	var registry *core.Registry
	var server *grpc.Server
	var listener *bufconn.Listener
	var unauthenticated *grpc.ClientConn
	var privateKey ed25519.PrivateKey
	var publicKey []byte
	var request *PairingRequest
	var device *ClientDevice
	var approved *PairingRequest
	var certificate tls.Certificate
	var config tls.Config
	var authenticated *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient

	var err error

	rpcTestSetup(t)
	identity, err = LoadServerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	registry = core.NewRegistry()
	server, err = NewServer(identity, registry)
	if err != nil {
		t.Fatal(err)
	}
	listener = bufconn.Listen(1 << 20)
	go server.Serve(listener)
	t.Cleanup(server.Stop)

	unauthenticated, err = grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, address string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs: identity.CAPool, ServerName: "localhost", MinVersion: tls.VersionTLS13,
		})))
	if err != nil {
		t.Fatal(err)
	}
	client = mininaruv1.NewMininaruServiceClient(unauthenticated)
	_, err = client.ListAgents(context.Background(), &mininaruv1.ListAgentsRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
	unauthenticated.Close()

	privateKey, publicKey = testPublicKey(t)
	request, err = PairingBegin("desktop", publicKey)
	if err != nil {
		t.Fatal(err)
	}
	device, err = PairingApprove(request.Code)
	if err != nil {
		t.Fatal(err)
	}
	approved, err = PairingGet(request.Id)
	if err != nil {
		t.Fatal(err)
	}
	certificate = testClientCertificate(t, approved.CertificatePEM, privateKey)
	config = tls.Config{Certificates: []tls.Certificate{certificate}, RootCAs: identity.CAPool,
		ServerName: "localhost", MinVersion: tls.VersionTLS13}

	authenticated, err = grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, address string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}), grpc.WithTransportCredentials(credentials.NewTLS(&config)))
	if err != nil {
		t.Fatal(err)
	}
	defer authenticated.Close()

	client = mininaruv1.NewMininaruServiceClient(authenticated)
	_, err = client.ListAgents(context.Background(), &mininaruv1.ListAgentsRequest{})
	if err != nil {
		t.Fatal(err)
	}

	err = ClientRevoke(device.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListAgents(context.Background(), &mininaruv1.ListAgentsRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("revoked code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestPairClientTrustsServerAndDialsWithIssuedIdentity(t *testing.T) {
	var identity *ServerIdentity
	var registry *core.Registry
	var server *grpc.Server
	var listener net.Listener
	var address string
	var fingerprint string
	var connection *grpc.ClientConn
	var client mininaruv1.MininaruServiceClient

	var err error

	rpcTestSetup(t)
	identity, err = LoadServerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	registry = core.NewRegistry()
	server, err = NewServer(identity, registry)
	if err != nil {
		t.Fatal(err)
	}
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	defer server.Stop()

	address = listener.Addr().String()
	fingerprint, err = ServerFingerprint(context.Background(), address)
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint != identity.Fingerprint {
		t.Fatalf("fingerprint = %s, want %s", fingerprint, identity.Fingerprint)
	}

	err = Pair(context.Background(), address, "laptop", fingerprint, func(request *PairingRequest) {
		_, err = PairingApprove(request.Code)
	})
	if err != nil {
		t.Fatal(err)
	}

	connection, err = Dial(context.Background(), address)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client = mininaruv1.NewMininaruServiceClient(connection)
	_, err = client.ListAgents(context.Background(), &mininaruv1.ListAgentsRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func chatRegistry(t *testing.T, upstream string) *core.Registry {
	var registry *core.Registry

	var err error

	t.Helper()

	core.Providers = nil
	core.DefaultProvider = nil
	core.Agents = nil
	core.Global = nil
	core.ProviderCreate(core.Provider{Name: "local", BaseURL: upstream, ApiKey: "key"})
	core.Global = core.AgentNew("naru", "", "", "model", core.Providers[0])
	err = core.ProviderSave()
	if err != nil {
		t.Fatal(err)
	}
	err = core.AgentSave()
	if err != nil {
		t.Fatal(err)
	}

	registry = core.NewRegistry()
	err = registry.Reload()
	if err != nil {
		t.Fatal(err)
	}

	return registry
}

func TestChatStreamsAndPersistsServerSession(t *testing.T) {
	var upstream *httptest.Server
	var registry *core.Registry
	var instance *core.Instance
	var session *core.Session
	var ctx context.Context
	var cancel context.CancelFunc
	var stream testChatStream
	var event *mininaruv1.ChatServerEvent
	var content string
	var reasoning string
	var completed *mininaruv1.ChatCompleted
	var messages []*core.Message

	var err error

	rpcTestSetup(t)
	config.Client = config.ClientConfig{Thinking: config.Thinking{Level: config.ThinkingOff}, Tools: config.Tools{Enabled: false}}

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chat\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning_content\":\"thought\",\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\n")
		io.WriteString(w, "data: {\"id\":\"chat\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model\",\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":1,\"total_tokens\":5}}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	registry = chatRegistry(t, upstream.URL)
	instance, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}
	session, err = instance.Session("remote")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	stream = testChatStream{ctx: ctx, incoming: make(chan *mininaruv1.ChatClientEvent, 1)}
	stream.incoming <- &mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Start{Start: &mininaruv1.ChatStart{
		SessionId: session.Id, Content: "hi", Thinking: config.ThinkingOff}}}

	err = (&mininaruService{registry: registry, slots: make(chan struct{}, 1)}).Chat(&stream)
	if err != nil {
		t.Fatal(err)
	}

	for _, event = range stream.outgoing {
		if event.GetContent() != nil {
			content = content + event.GetContent().GetText()
		}
		if event.GetReasoning() != nil {
			reasoning = reasoning + event.GetReasoning().GetText()
		}
		if event.GetCompleted() != nil {
			completed = event.GetCompleted()
		}
	}
	if content != "hello" {
		t.Fatalf("content = %q, want hello", content)
	}
	if reasoning != "thought" {
		t.Fatalf("reasoning = %q, want thought", reasoning)
	}
	if completed == nil || completed.GetMessage().GetContent() != "hello" || completed.GetUsage().GetTotalTokens() != 5 {
		t.Fatalf("completed = %#v", completed)
	}

	messages, err = core.MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Content != "hi" || messages[1].Content != "hello" {
		t.Fatalf("persisted messages = %#v", messages)
	}
}

func TestChatCarriesToolApprovalOverTheStream(t *testing.T) {
	var calls atomic.Int32
	var upstream *httptest.Server
	var registry *core.Registry
	var instance *core.Instance
	var session *core.Session
	var ctx context.Context
	var cancel context.CancelFunc
	var stream testChatStream
	var event *mininaruv1.ChatServerEvent
	var requested bool
	var completed *mininaruv1.ChatCompleted

	var err error

	rpcTestSetup(t)
	config.Client = config.ClientConfig{Thinking: config.Thinking{Level: config.ThinkingOff}, Tools: config.Tools{Enabled: true}}

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			io.WriteString(w, "data: {\"id\":\"tool\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"bash_exec\",\"arguments\":\"{\\\"command\\\":\\\"echo unsafe\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
			io.WriteString(w, "data: [DONE]\n\n")
			return
		}

		io.WriteString(w, "data: {\"id\":\"answer\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"denied safely\"},\"finish_reason\":\"stop\"}]}\n\n")
		io.WriteString(w, "data: {\"id\":\"answer\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model\",\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2,\"total_tokens\":10}}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	registry = chatRegistry(t, upstream.URL)
	instance, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}
	session, err = instance.Session("approval")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	stream = testChatStream{ctx: ctx, incoming: make(chan *mininaruv1.ChatClientEvent, 1), autoDeny: true}
	stream.incoming <- &mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Start{Start: &mininaruv1.ChatStart{
		SessionId: session.Id, Content: "run it", Thinking: config.ThinkingOff}}}

	err = (&mininaruService{registry: registry, slots: make(chan struct{}, 1)}).Chat(&stream)
	if err != nil {
		t.Fatal(err)
	}
	for _, event = range stream.outgoing {
		if event.GetApproval() != nil && event.GetApproval().GetToolName() == "bash_exec" {
			requested = true
		}
		if event.GetCompleted() != nil {
			completed = event.GetCompleted()
		}
	}
	if !requested {
		t.Fatal("dangerous tool produced no approval request")
	}
	if completed == nil || completed.GetMessage().GetContent() != "denied safely" {
		t.Fatalf("completed = %#v", completed)
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want tool round and answer round", calls.Load())
	}
}

func TestChatCancellationReachesTheModelTurn(t *testing.T) {
	var started chan struct{}
	var release chan struct{}
	var upstream *httptest.Server
	var registry *core.Registry
	var instance *core.Instance
	var session *core.Session
	var ctx context.Context
	var cancel context.CancelFunc
	var stream testChatStream
	var result chan error
	var statusValue string

	var err error

	rpcTestSetup(t)
	config.Client = config.ClientConfig{Thinking: config.Thinking{Level: config.ThinkingOff}, Tools: config.Tools{Enabled: false}}
	started = make(chan struct{})
	release = make(chan struct{})
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer func() {
		close(release)
		upstream.Close()
	}()

	registry = chatRegistry(t, upstream.URL)
	instance, err = registry.Get("naru")
	if err != nil {
		t.Fatal(err)
	}
	session, err = instance.Session("cancel")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	stream = testChatStream{ctx: ctx, incoming: make(chan *mininaruv1.ChatClientEvent, 1)}
	stream.incoming <- &mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Start{Start: &mininaruv1.ChatStart{
		SessionId: session.Id, Content: "wait", Thinking: config.ThinkingOff}}}
	result = make(chan error, 1)
	go func() {
		result <- (&mininaruService{registry: registry, slots: make(chan struct{}, 1)}).Chat(&stream)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("model request did not start")
	}
	stream.incoming <- &mininaruv1.ChatClientEvent{Event: &mininaruv1.ChatClientEvent_Cancel{Cancel: &mininaruv1.Empty{}}}

	select {
	case err = <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled chat did not stop")
	}

	err = util.DB.QueryRow("SELECT status FROM messages WHERE session_id = ?;", session.Id).Scan(&statusValue)
	if err != nil {
		t.Fatal(err)
	}
	if statusValue != core.MessageCancelled {
		t.Fatalf("message status = %s, want %s", statusValue, core.MessageCancelled)
	}
}
