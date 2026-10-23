package api

import (
	"encoding/json"
	"net/http"
)

// configurationUpdate implements upstream's explicit operator-owned mode.
// The response must never claim that a panel's Linux configuration was applied.
func (a *API) configurationUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.Manager.Config.IgnorePanelConfigUpdates {
		failure(w, http.StatusNotImplemented, "Remote configuration changes are unsupported; enable ignore_panel_config_updates locally for operator-managed configuration")
		return
	}
	var payload map[string]json.RawMessage
	if !decode(w, r, &payload) {
		return
	}
	if payload == nil {
		failure(w, http.StatusBadRequest, "Expected a configuration object")
		return
	}
	respond(w, http.StatusOK, map[string]bool{"applied": false})
}
