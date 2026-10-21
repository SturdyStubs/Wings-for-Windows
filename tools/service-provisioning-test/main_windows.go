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
		if len(c.Profiles[id].EggConfigFiles) > 2 {
			server.ProcessConfiguration.Configs = append(server.ProcessConfiguration.Configs, json.RawMessage(`{"file":"server.properties","parser":"properties","replace":[{"match":"server-port","replace_with":28015},{"match":"name","replace_with":"Windows Café 😀"},{"match":"mode","if_value":"old","replace_with":"new"},{"match":"absent","if_value":"old","replace_with":"no"},{"match":"added","replace_with":"${USERNAME}"},{"match":"enabled","replace_with":true}]}`))
		}
	}
	if len(c.Profiles[id].EggConfigFiles) > 3 {
		server.ProcessConfiguration.Configs = append(server.ProcessConfiguration.Configs, json.RawMessage(`{"file":"server.ini","parser":"ini","replace":[{"match":"port","replace_with":28015},{"match":"network.enabled","replace_with":true},{"match":"[parent.child].shared","replace_with":"child-only"},{"match":"[/Script.Game.Settings].Server.Name","replace_with":"Café"},{"match":"added","replace_with":"${USERNAME}"},{"match":"network.note","replace_with":"  ;# keep  "}]}`))
	}
	if len(c.Profiles[id].EggConfigFiles) > 4 {
		server.ProcessConfiguration.Configs = append(server.ProcessConfiguration.Configs, json.RawMessage(`{"file":"server.yml","parser":"yaml","replace":[{"match":"network.port","replace_with":"28015"},{"match":"network.enabled","replace_with":true},{"match":"name","replace_with":"Café"},{"match":"worlds.*.mode","if_value":"old","replace_with":"new"},{"match":"added","replace_with":"${USERNAME}"}]}`))
	}
	if len(c.Profiles[id].EggConfigFiles) > 5 {
		server.ProcessConfiguration.Configs = append(server.ProcessConfiguration.Configs, json.RawMessage(`{"file":"server.xml","parser":"xml","replace":[{"match":"server.port","replace_with":28015},{"match":"server.name","replace_with":"Café & <safe>"},{"match":"server.worlds.world.mode","replace_with":"new"},{"match":"server.worlds.world","replace_with":"[enabled='true']"},{"match":"server.literal","replace_with":"${USERNAME}"}]}`))
	}
	if c.ConsoleLogs != nil {
		server.Settings.Environment["LOG_PROBE_PATH"] = filepath.Join(c.ConsoleLogs.Directory, id, "console.jsonl")
		if p := c.Profiles[id].CrashRecovery; p != nil && p.PersistHistory {
			server.Settings.Environment["RECOVERY_PROBE_PATH"] = filepath.Join(c.ConsoleLogs.Directory, id, "recovery.1.json")
			if p := c.Profiles[id]; p.RestoreRunning {
				server.Settings.Environment["INTENT_PROBE_PATH"] = filepath.Join(c.ConsoleLogs.Directory, id, "intent.1.json")
			}
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
