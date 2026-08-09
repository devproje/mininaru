// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	var out bytes.Buffer

	var err error

	t.Helper()

	err = util.LogInit(util.LogOptions{Level: "debug", Format: util.LogFormatJSON, Output: &out})
	if err != nil {
		t.Fatal(err)
	}

	return &out
}

func logRecords(t *testing.T, out *bytes.Buffer) []map[string]any {
	var line string
	var record map[string]any
	var records []map[string]any

	var err error

	t.Helper()

	for _, line = range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}

		record = nil
		err = json.Unmarshal([]byte(line), &record)
		if err != nil {
			t.Fatalf("log line is not json: %s", line)
		}

		records = append(records, record)
	}

	return records
}

func findRecord(records []map[string]any, msg string) map[string]any {
	var record map[string]any

	for _, record = range records {
		if record["msg"] == msg {
			return record
		}
	}

	return nil
}

func TestRequestLoggingRecordsOutcome(t *testing.T) {
	var out *bytes.Buffer
	var upstream *httptest.Server
	var reg *core.Registry
	var handler http.Handler
	var recorder *httptest.ResponseRecorder
	var completed map[string]any

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(streamChunk(`{"content":"hi"}`, `"stop"`)))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	reg = setupAgent(t, upstream.URL)
	out = captureLogs(t)
	handler = routes("secret", reg)

	recorder = request(t, handler, http.MethodPost, pathCompletions, "secret",
		`{"model":"naru","messages":[{"role":"user","content":"hello"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	completed = findRecord(logRecords(t, out), "request completed")
	if completed == nil {
		t.Fatalf("no completion record in:\n%s", out.String())
	}

	if completed["method"] != http.MethodPost {
		t.Fatalf("method = %v", completed["method"])
	}
	if completed["path"] != pathCompletions {
		t.Fatalf("path = %v", completed["path"])
	}
	if completed["status"] != float64(http.StatusOK) {
		t.Fatalf("status = %v", completed["status"])
	}
	if completed["request_id"] == nil || completed["request_id"] == "" {
		t.Fatal("record carries no request_id")
	}
	if completed["duration_ms"] == nil {
		t.Fatal("record carries no duration_ms")
	}
}

func TestRequestLoggingCorrelatesWithTheResponseHeader(t *testing.T) {
	var out *bytes.Buffer
	var reg *core.Registry
	var handler http.Handler
	var recorder *httptest.ResponseRecorder
	var rejected map[string]any

	reg = core.NewRegistry()
	out = captureLogs(t)
	handler = routes("secret", reg)

	recorder = request(t, handler, http.MethodGet, pathModels, "wrong", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}

	rejected = findRecord(logRecords(t, out), "request rejected")
	if rejected == nil {
		t.Fatalf("no rejection record in:\n%s", out.String())
	}

	if rejected["request_id"] != recorder.Header().Get(requestIdHeader) {
		t.Fatalf("log request_id %v does not match the %s header %q",
			rejected["request_id"], requestIdHeader, recorder.Header().Get(requestIdHeader))
	}

	if findRecord(logRecords(t, out), "rejected an unauthorized request") == nil {
		t.Fatalf("no auth failure record in:\n%s", out.String())
	}
}

func TestRequestLoggingKeepsAnInboundRequestId(t *testing.T) {
	var out *bytes.Buffer
	var reg *core.Registry
	var handler http.Handler
	var recorder *httptest.ResponseRecorder
	var req *http.Request
	var completed map[string]any

	reg = core.NewRegistry()
	out = captureLogs(t)
	handler = routes("secret", reg)

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, pathModels, nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(requestIdHeader, "trace-abc")

	handler.ServeHTTP(recorder, req)

	if recorder.Header().Get(requestIdHeader) != "trace-abc" {
		t.Fatalf("%s header = %q", requestIdHeader, recorder.Header().Get(requestIdHeader))
	}

	completed = findRecord(logRecords(t, out), "request completed")
	if completed == nil || completed["request_id"] != "trace-abc" {
		t.Fatalf("inbound request id was not reused:\n%s", out.String())
	}
}

func TestStreamingStillFlushesThroughTheLoggingWrapper(t *testing.T) {
	var out *bytes.Buffer
	var upstream *httptest.Server
	var reg *core.Registry
	var handler http.Handler
	var recorder *httptest.ResponseRecorder
	var finished map[string]any

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(streamChunk(`{"content":"hi"}`, `"stop"`)))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	reg = setupAgent(t, upstream.URL)
	out = captureLogs(t)
	handler = routes("secret", reg)

	recorder = request(t, handler, http.MethodPost, pathCompletions, "secret",
		`{"model":"naru","messages":[{"role":"user","content":"hello"}],"stream":true}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !containsAll(recorder.Body.String(), "data: ", "[DONE]") {
		t.Fatalf("streaming body = %s", recorder.Body.String())
	}

	finished = findRecord(logRecords(t, out), "completion finished")
	if finished == nil {
		t.Fatalf("no completion record in:\n%s", out.String())
	}
	if finished["stream"] != true {
		t.Fatalf("stream = %v, want true", finished["stream"])
	}
	if finished["agent"] != "naru" {
		t.Fatalf("agent = %v, want naru", finished["agent"])
	}
}
