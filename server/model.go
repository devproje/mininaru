package server

import (
	"net/http"
	"time"

	"github.com/devproje/mininaru/core"
)

func providerLabel(id string) string {
	var prov *core.Provider

	var err error

	prov, err = core.ProviderFind(id)
	if err != nil {
		return "mininaru"
	}

	return prov.Name
}

func modelList(reg *core.Registry) ModelList {
	var list ModelList
	var cur *core.Instance
	var created int64

	created = time.Now().Unix()
	list.Object = objectList

	for _, cur = range reg.List() {
		list.Data = append(list.Data, Model{
			Id:      cur.Agent.Name,
			Object:  objectModel,
			Created: created,
			OwnedBy: providerLabel(cur.Agent.ProviderId),
		})
	}

	if list.Data == nil {
		list.Data = []Model{}
	}

	return list
}

func handleModels(w http.ResponseWriter, r *http.Request, reg *core.Registry) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "only GET is supported")
		return
	}

	writeJSON(w, http.StatusOK, modelList(reg))
}
