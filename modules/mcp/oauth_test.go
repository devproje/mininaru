// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package mcp

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"golang.org/x/oauth2"
)

func TestCredentialStoreRoundTripsAndEncryptsSecrets(t *testing.T) {
	var cred oauthCred
	var loaded oauthCred
	var found bool
	var raw []byte

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cred = oauthCred{
		Server: "notion", ClientID: "client-id", ClientSecret: "client-secret",
		AuthURL: "https://idp.example/authorize", TokenURL: "https://idp.example/token",
		AccessToken: "access-token", RefreshToken: "refresh-token", TokenType: "Bearer",
	}

	err = saveCredential(cred)
	if err != nil {
		t.Fatal(err)
	}

	raw, err = os.ReadFile(util.Path(oauthStorePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "enc:v1:") || strings.Contains(string(raw), "client-secret") {
		t.Fatalf("mcp_oauth.json was not encrypted at rest: %s", raw)
	}

	loaded, found, err = credentialFor("notion")
	if err != nil {
		t.Fatal(err)
	}
	if !found || loaded.ClientSecret != "client-secret" || loaded.AccessToken != "access-token" {
		t.Fatalf("credentialFor = %+v, want the saved credential back", loaded)
	}

	err = deleteCredential("notion")
	if err != nil {
		t.Fatal(err)
	}

	_, found, err = credentialFor("notion")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("credentialFor found a credential after deleteCredential")
	}
}

func TestHasAuthorizationHeaderIsCaseInsensitive(t *testing.T) {
	if hasAuthorizationHeader(nil) {
		t.Fatal("hasAuthorizationHeader(nil) = true")
	}
	if hasAuthorizationHeader(map[string]string{"X-Api-Key": "x"}) {
		t.Fatal("hasAuthorizationHeader found an unrelated header")
	}
	if !hasAuthorizationHeader(map[string]string{"authorization": "Bearer x"}) {
		t.Fatal("hasAuthorizationHeader missed a lowercase header")
	}
}

type stubTokenSource struct {
	token *oauth2.Token
}

func (s *stubTokenSource) Token() (*oauth2.Token, error) {
	return s.token, nil
}

func TestPersistingTokenSourceSavesOnceThenDedupes(t *testing.T) {
	var source persistingTokenSource
	var token *oauth2.Token
	var firstWrite time.Time
	var secondWrite time.Time
	var info os.FileInfo

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	source = persistingTokenSource{
		cred: oauthCred{Server: "notion"},
		base: &stubTokenSource{token: &oauth2.Token{AccessToken: "first", Expiry: time.Now().Add(time.Hour)}},
	}

	token, err = source.Token()
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "first" {
		t.Fatalf("Token() = %+v", token)
	}

	info, err = os.Stat(util.Path(oauthStorePath))
	if err != nil {
		t.Fatal(err)
	}
	firstWrite = info.ModTime()

	_, err = source.Token()
	if err != nil {
		t.Fatal(err)
	}

	info, err = os.Stat(util.Path(oauthStorePath))
	if err != nil {
		t.Fatal(err)
	}
	secondWrite = info.ModTime()

	if !secondWrite.Equal(firstWrite) {
		t.Fatal("an unchanged token was persisted a second time")
	}
}

func TestNewOAuthHandlerPrefersAStoredClientOverDynamicRegistration(t *testing.T) {
	var handler auth.OAuthHandler

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	handler, err = newOAuthHandler(&Server{Name: "fresh", Transport: TransportHTTP, URL: "https://mcp.example/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if handler == nil {
		t.Fatal("newOAuthHandler returned a nil handler with no stored credential")
	}

	err = saveCredential(oauthCred{Server: "returning", ClientID: "stored-client-id"})
	if err != nil {
		t.Fatal(err)
	}

	handler, err = newOAuthHandler(&Server{Name: "returning", Transport: TransportHTTP, URL: "https://mcp.example/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if handler == nil {
		t.Fatal("newOAuthHandler returned a nil handler with a stored credential")
	}
}
