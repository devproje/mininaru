// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/core"
)

func TestModelListExposesGlobalAndNamedAgents(t *testing.T) {
	var reg *core.Registry
	var recorder *httptest.ResponseRecorder
	var list ModelList

	var err error

	reg = setupAgent(t, "http://127.0.0.1")
	core.Agents = append(core.Agents, core.AgentNew("helper", "", "", "qwen", core.Providers[0]))
	reloadRegistry(t, reg)

	recorder = request(t, routes("k", reg), http.MethodGet, pathModels, "k", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("models = %d, want 200", recorder.Code)
	}

	err = json.Unmarshal(recorder.Body.Bytes(), &list)
	if err != nil {
		t.Fatal(err)
	}

	if list.Object != objectList || len(list.Data) != 2 {
		t.Fatalf("model list = %#v", list)
	}

	if list.Data[0].Id != "naru" || list.Data[1].Id != "helper" {
		t.Fatalf("model ids = %q, %q", list.Data[0].Id, list.Data[1].Id)
	}

	if list.Data[0].OwnedBy != "local" || list.Data[0].Object != objectModel {
		t.Fatalf("model metadata = %#v", list.Data[0])
	}
}
