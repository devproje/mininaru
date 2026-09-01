// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

const (
	DefaultUrl  string        = "ws://127.0.0.1:8223/ws"
	dialTimeout time.Duration = 3 * time.Second
)

type Frame struct {
	Type      string   `json:"type,omitempty"`
	SessionId string   `json:"session_id"`
	Content   string   `json:"content,omitempty"`
	Cwd       string   `json:"cwd,omitempty"`
	Decision  string   `json:"decision,omitempty"`
	Images    []string `json:"images,omitempty"`
}

type Reply struct {
	Type      string                      `json:"type"`
	SessionId string                      `json:"session_id"`
	Chunk     *openai.ChatCompletionChunk `json:"chunk,omitempty"`
	Reasoning string                      `json:"reasoning,omitempty"`
	Message   string                      `json:"message,omitempty"`
	Name      string                      `json:"name,omitempty"`
	Status    string                      `json:"status,omitempty"`
	Arguments string                      `json:"arguments,omitempty"`
}

func ApiBase(endpoint string) (string, error) {
	var parsed *url.URL

	var err error

	parsed, err = url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	}

	parsed.Path = "/api"
	parsed.RawQuery = ""

	return parsed.String(), nil
}

func Api(method string, endpoint string, apiKey string, payload any, out any) error {
	var body []byte
	var reader io.Reader
	var req *http.Request
	var res *http.Response

	var err error

	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(body)
	}

	req, err = http.NewRequest(method, endpoint, reader)
	if err != nil {
		return err
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	return json.Unmarshal(body, out)
}

func Upload(base string, apiKey string, sessionId string, path string) (string, error) {
	var file *os.File
	var body bytes.Buffer
	var writer *multipart.Writer
	var part io.Writer
	var req *http.Request
	var res *http.Response
	var raw []byte
	var parsed struct {
		Id string `json:"id"`
	}

	var err error

	file, err = os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer = multipart.NewWriter(&body)

	part, err = writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	req, err = http.NewRequest(http.MethodPost, base+"/sessions/"+sessionId+"/attachments", &body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	raw, err = io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(raw)))
	}

	err = json.Unmarshal(raw, &parsed)
	if err != nil {
		return "", err
	}

	return parsed.Id, nil
}

func Agent(base string, apiKey string, name string) (*core.Agent, error) {
	var list []*core.Agent
	var item *core.Agent

	var err error

	err = Api(http.MethodGet, base+"/agents", apiKey, nil, &list)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("no agent is registered on the server")
	}

	if name == "" {
		return list[0], nil
	}

	for _, item = range list {
		if item.Id == name || item.Name == name {
			return item, nil
		}
	}

	return nil, fmt.Errorf("agent %q not found", name)
}

func Session(base string, apiKey string, seed string, agent string) (*core.Session, error) {
	var session core.Session
	var created core.Session
	var target *core.Agent

	var err error

	if seed != "" {
		err = Api(http.MethodGet, base+"/sessions/"+seed, apiKey, nil, &session)
		if err != nil {
			return nil, err
		}

		return &session, nil
	}

	target, err = Agent(base, apiKey, agent)
	if err != nil {
		return nil, err
	}

	err = Api(http.MethodPost, base+"/sessions", apiKey, map[string]string{"agent_id": target.Id}, &created)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func isLoopbackUrl(endpoint string) bool {
	var parsed *url.URL
	var host string

	var err error

	parsed, err = url.Parse(endpoint)
	if err != nil {
		return false
	}

	host = parsed.Hostname()

	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func ResolveApiKey(explicit string, endpoint string) string {
	var fromEnv string
	var fromFile []byte

	var err error

	if explicit != "" {
		return explicit
	}

	fromEnv = os.Getenv("MININARU_API_KEY")
	if fromEnv != "" {
		return fromEnv
	}

	if !isLoopbackUrl(endpoint) {
		return ""
	}

	fromFile, err = os.ReadFile(util.Path("mininaru.key"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(fromFile))
}

func ResolveCwd(dir string) (string, error) {
	var info os.FileInfo

	var err error

	if dir == "" {
		return os.Getwd()
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	info, err = os.Stat(dir)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return "", fmt.Errorf("--cwd %s is not a directory", dir)
	}

	return dir, nil
}

func Pump(conn *websocket.Conn) <-chan Reply {
	var out chan Reply

	out = make(chan Reply, 64)

	go func() {
		var reply Reply

		for {
			reply = Reply{}

			if conn.ReadJSON(&reply) != nil {
				close(out)

				return
			}

			out <- reply
		}
	}()

	return out
}

func Dial(endpoint string, apiKey string) (*websocket.Conn, error) {
	var dialer websocket.Dialer
	var header http.Header
	var conn *websocket.Conn

	var err error

	dialer = websocket.Dialer{HandshakeTimeout: dialTimeout}

	if apiKey != "" {
		header = http.Header{"Authorization": []string{"Bearer " + apiKey}}
	}

	conn, _, err = dialer.Dial(endpoint, header)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
