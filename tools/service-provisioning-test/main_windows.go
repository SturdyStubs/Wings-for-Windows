package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(2)
	}
	c, err := config.Load(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	id := ""
	for key := range c.Profiles {
		id = key
		break
	}
	server := node.RemoteServer{UUID: id, Settings: node.Settings{UUID: id, Environment: map[string]any{}}}
	if len(c.Profiles[id].EggConfigFiles) > 0 {
		server.ProcessConfiguration = &node.ProcessConfiguration{}
		server.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		server.ProcessConfiguration.Stop.Type = "command"
		server.ProcessConfiguration.Stop.Value = "stop"
		server.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"server.name","replace_with":"server.name \"Windows Café\""},{"match":"server.port","replace_with":"server.port 28015"}]}`)}
		if len(c.Profiles[id].EggConfigFiles) > 1 {
			server.ProcessConfiguration.Configs = append(server.ProcessConfiguration.Configs, json.RawMessage(`{"file":"server.json","parser":"json","replace":[{"match":"network.port","replace_with":"28015"},{"match":"network.enabled","replace_with":true},{"match":"name","replace_with":"Café"},{"match":"worlds.*.mode","if_value":"old","replace_with":"new"}]}`))
		}
	}
	if c.ConsoleLogs != nil {
		server.Settings.Environment["LOG_PROBE_PATH"] = filepath.Join(c.ConsoleLogs.Directory, id, "console.jsonl")
		if p := c.Profiles[id].CrashRecovery; p != nil && p.PersistHistory {
			server.Settings.Environment["RECOVERY_PROBE_PATH"] = filepath.Join(c.ConsoleLogs.Directory, id, "recovery.1.json")
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/remote/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+c.TokenID+"."+c.Token {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []node.RemoteServer{server}, "meta": map[string]int{"last_page": 1}})
	})
	mux.HandleFunc("/api/remote/servers/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+c.TokenID+"."+c.Token {
			w.WriteHeader(401)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/api/remote/servers/") != id {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(server)
	})
	addr := strings.TrimPrefix(c.PanelURL, "http://")
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		log.Fatal("fixture requires literal IPv4 loopback")
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}
