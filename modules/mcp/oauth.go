// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

type oauthCred struct {
	Server       string    `json:"server"`
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	AuthURL      string    `json:"auth_url,omitempty"`
	TokenURL     string    `json:"token_url,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

type oauthStore struct {
	Entries []oauthCred `json:"entries"`
}

type callbackResult struct {
	code  string
	state string
	iss   string
	err   error
}

type persistingTokenSource struct {
	mu   sync.Mutex
	cred oauthCred
	base oauth2.TokenSource
	last string
}

const oauthStorePath = "mcp_oauth.json"
const oauthCallbackAddr = "127.0.0.1:48415"
const oauthRedirectURL = "http://" + oauthCallbackAddr + "/callback"
const oauthLoginTimeout = 5 * time.Minute
const oauthCallbackPage = "<html><body>mininaru is logged in, you can close this tab.</body></html>"

func transformCred(cred oauthCred, transform func(string) (string, error)) (oauthCred, error) {
	var err error

	cred.ClientSecret, err = transform(cred.ClientSecret)
	if err != nil {
		return oauthCred{}, err
	}

	cred.AccessToken, err = transform(cred.AccessToken)
	if err != nil {
		return oauthCred{}, err
	}

	cred.RefreshToken, err = transform(cred.RefreshToken)
	if err != nil {
		return oauthCred{}, err
	}

	return cred, nil
}

func loadOAuthStore() (oauthStore, error) {
	var path string
	var buf []byte
	var store oauthStore
	var index int

	var err error

	path = util.Path(oauthStorePath)

	buf, err = os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return oauthStore{}, nil
		}

		return oauthStore{}, err
	}

	err = json.Unmarshal(buf, &store)
	if err != nil {
		return oauthStore{}, err
	}

	for index = range store.Entries {
		store.Entries[index], err = transformCred(store.Entries[index], util.Decrypt)
		if err != nil {
			return oauthStore{}, err
		}
	}

	return store, nil
}

func saveOAuthStore(store oauthStore) error {
	var path string
	var buf []byte
	var encrypted oauthStore
	var index int

	var err error

	encrypted.Entries = make([]oauthCred, len(store.Entries))

	for index = range store.Entries {
		encrypted.Entries[index], err = transformCred(store.Entries[index], util.Encrypt)
		if err != nil {
			return err
		}
	}

	path = util.Path(oauthStorePath)

	buf, err = json.MarshalIndent(encrypted, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, buf, 0600)
}

func credentialFor(name string) (oauthCred, bool, error) {
	var store oauthStore
	var index int

	var err error

	store, err = loadOAuthStore()
	if err != nil {
		return oauthCred{}, false, err
	}

	for index = range store.Entries {
		if store.Entries[index].Server == name {
			return store.Entries[index], true, nil
		}
	}

	return oauthCred{}, false, nil
}

func saveCredential(cred oauthCred) error {
	var store oauthStore
	var index int
	var found bool

	var err error

	store, err = loadOAuthStore()
	if err != nil {
		return err
	}

	for index = range store.Entries {
		if store.Entries[index].Server == cred.Server {
			store.Entries[index] = cred
			found = true
			break
		}
	}
	if !found {
		store.Entries = append(store.Entries, cred)
	}

	return saveOAuthStore(store)
}

func deleteCredential(name string) error {
	var store oauthStore
	var index int
	var kept []oauthCred

	var err error

	store, err = loadOAuthStore()
	if err != nil {
		return err
	}

	for index = range store.Entries {
		if store.Entries[index].Server != name {
			kept = append(kept, store.Entries[index])
		}
	}

	store.Entries = kept

	return saveOAuthStore(store)
}

func Logout(name string) error {
	return deleteCredential(name)
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	var token *oauth2.Token

	var err error

	token, err = p.base.Token()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if token.AccessToken == p.last {
		return token, nil
	}

	p.cred.AccessToken = token.AccessToken
	p.cred.RefreshToken = token.RefreshToken
	p.cred.TokenType = token.TokenType
	p.cred.Expiry = token.Expiry

	err = saveCredential(p.cred)
	if err != nil {
		util.Log.Warn("could not persist an mcp oauth token", "server", p.cred.Server, "error", err)
		return token, nil
	}

	p.last = token.AccessToken

	return token, nil
}

func wrapTokenSource(cred oauthCred, base oauth2.TokenSource) oauth2.TokenSource {
	return &persistingTokenSource{cred: cred, base: base}
}

func openURL(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

func callbackHandler(ch chan callbackResult) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var result callbackResult
		var query url.Values
		var message string

		query = r.URL.Query()
		result.code = query.Get("code")
		result.state = query.Get("state")
		result.iss = query.Get("iss")

		message = query.Get("error")
		if message != "" {
			result.err = fmt.Errorf("authorization server returned %q: %s", message, query.Get("error_description"))
		}

		fmt.Fprint(w, oauthCallbackPage)
		ch <- result
	}
}

func fetchAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	var listener net.Listener
	var server http.Server
	var ch chan callbackResult
	var result callbackResult

	var err error

	listener, err = net.Listen("tcp", oauthCallbackAddr)
	if err != nil {
		return nil, fmt.Errorf("could not open the mcp oauth callback listener: %w", err)
	}

	ch = make(chan callbackResult, 1)
	server.Handler = callbackHandler(ch)

	go server.Serve(listener)
	defer server.Close()

	err = openURL(args.URL)
	if err != nil {
		util.Log.Warn("could not open a browser for mcp login, open the url manually", "url", args.URL, "error", err)
	} else {
		util.Log.Info("opened a browser for mcp login")
	}

	select {
	case result = <-ch:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(oauthLoginTimeout):
		return nil, fmt.Errorf("timed out waiting for the mcp login callback")
	}

	if result.err != nil {
		return nil, result.err
	}

	return &auth.AuthorizationResult{Code: result.code, State: result.state, Iss: result.iss}, nil
}

func newOAuthHandler(entry *Server) (auth.OAuthHandler, error) {
	var cred oauthCred
	var found bool
	var secretAuth *oauthex.ClientSecretAuth
	var config auth.AuthorizationCodeHandlerConfig
	var metadata oauthex.ClientRegistrationMetadata

	var err error

	cred, found, err = credentialFor(entry.Name)
	if err != nil {
		return nil, err
	}

	config.RedirectURL = oauthRedirectURL
	config.AuthorizationCodeFetcher = fetchAuthorizationCode
	config.RequestRefreshToken = true
	config.NewTokenSource = func(tsCtx context.Context, cfg *oauth2.Config, token *oauth2.Token) (oauth2.TokenSource, error) {
		return wrapTokenSource(oauthCred{
			Server:       entry.Name,
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			AuthURL:      cfg.Endpoint.AuthURL,
			TokenURL:     cfg.Endpoint.TokenURL,
		}, cfg.TokenSource(tsCtx, token)), nil
	}

	if found && cred.ClientID != "" {
		if cred.ClientSecret != "" {
			secretAuth = &oauthex.ClientSecretAuth{ClientSecret: cred.ClientSecret}
		}

		config.PreregisteredClient = &oauthex.ClientCredentials{ClientID: cred.ClientID, ClientSecretAuth: secretAuth}
	} else {
		metadata.RedirectURIs = []string{oauthRedirectURL}
		metadata.GrantTypes = []string{"authorization_code", "refresh_token"}
		metadata.TokenEndpointAuthMethod = "none"
		metadata.ApplicationType = "native"
		metadata.ClientName = "mininaru"

		config.DynamicClientRegistrationConfig = &auth.DynamicClientRegistrationConfig{Metadata: &metadata}
	}

	if found && cred.AuthURL != "" && cred.TokenURL != "" && (cred.AccessToken != "" || cred.RefreshToken != "") {
		config.InitialTokenSource = wrapTokenSource(cred, (&oauth2.Config{
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			Endpoint:     oauth2.Endpoint{AuthURL: cred.AuthURL, TokenURL: cred.TokenURL},
		}).TokenSource(context.Background(), &oauth2.Token{
			AccessToken:  cred.AccessToken,
			RefreshToken: cred.RefreshToken,
			TokenType:    cred.TokenType,
			Expiry:       cred.Expiry,
		}))
	}

	return auth.NewAuthorizationCodeHandler(&config)
}

func serverIndex(name string) int {
	var index int

	for index = range Loaded.Servers {
		if Loaded.Servers[index].Name == name {
			return index
		}
	}

	return -1
}

func Login(ctx context.Context, name string) error {
	var index int
	var entry Server
	var current *session
	var inOrder bool
	var existing string

	var err error

	err = Load()
	if err != nil {
		return err
	}

	index = serverIndex(name)
	if index < 0 {
		return fmt.Errorf("mcp server %q not found", name)
	}

	entry = Loaded.Servers[index]
	if entry.Transport != TransportHTTP {
		return fmt.Errorf("mcp server %q is not an http server", name)
	}
	if hasAuthorizationHeader(entry.Headers) {
		return fmt.Errorf("mcp server %q already uses a manual authorization header", name)
	}

	current = dial(ctx, entry)
	if current.err != nil {
		return current.err
	}

	shared.mu.Lock()

	if shared.sessions[name] != nil && shared.sessions[name].client != nil {
		shared.sessions[name].client.Close()
	}

	shared.sessions[name] = current

	for _, existing = range shared.order {
		if existing == name {
			inOrder = true
			break
		}
	}
	if !inOrder {
		shared.order = append(shared.order, name)
	}

	shared.rebind()
	shared.mu.Unlock()

	return nil
}
