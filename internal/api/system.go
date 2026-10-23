package api

import (
	"net/http"
	"wings-windows/internal/config"
	"wings-windows/internal/native"
)

func (a *API) system(w http.ResponseWriter, r *http.Request) {
	info, err := native.HostInfo(a.Manager.Config.DataDirectory)
	if err != nil {
		failure(w, 500, "Cannot read Windows host information")
		return
	}
	capabilities := []string{"native_executable", "job_objects", "console", "power", "files_read_write", "files_create_directory", "files_rename_move", "files_copy", "files_delete", "signed_file_download", "signed_file_upload", "files_compress_tar_gzip", "live_resource_limits", "egg_process_opt_in", "native_installer_opt_in"}
	if a.Manager.Config.IgnorePanelConfigUpdates {
		capabilities = append(capabilities, "configuration_updates_operator_managed")
	}
	if a.Manager.Config.AllowPendingCreation {
		capabilities = append(capabilities, "pending_creation_manual_approval")
	}
	if r.URL.Query().Get("v") != "2" {
		respond(w, 200, map[string]any{"architecture": info.Architecture, "cpu_count": info.CPUThreads, "kernel_version": info.KernelVersion, "os": info.OSType, "version": config.Version, "capabilities": capabilities})
		return
	}
	counts := map[string]int{"total": 0, "running": 0, "starting": 0, "stopping": 0, "stopped": 0}
	for _, s := range a.Manager.All() {
		counts["total"]++
		state := s.State()
		if state == "offline" {
			state = "stopped"
		}
		counts[state]++
	}
	respond(w, 200, map[string]any{"version": config.Version, "system": info, "docker": map[string]any{"version": "unavailable", "cgroups": map[string]string{"driver": "none", "version": "none"}, "containers": map[string]int{"total": 0, "running": 0, "paused": 0, "stopped": 0}, "storage": map[string]string{"driver": "unavailable", "filesystem": "unavailable"}, "runc": map[string]string{"version": "unavailable"}}, "native": map[string]any{"runtime": "windows_job_objects", "capabilities": capabilities, "servers": counts, "data_filesystem": info.Filesystem, "container_isolation": false}})
}
