package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
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
