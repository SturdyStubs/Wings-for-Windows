// On-node opt-in SFTP probe: a temporary panel SSH key, never a new account.
package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func need(ok bool) {
	if !ok {
		panic("SFTP probe failed; sensitive details omitted")
	}
}
func nonce() string {
	var b [16]byte
	_, e := rand.Read(b[:])
	need(e == nil)
	return hex.EncodeToString(b[:])
}
func main() {
	if len(os.Args) == 3 && os.Args[1] == "--generate-host-key" {
		_, k, e := ed25519.GenerateKey(rand.Reader)
		need(e == nil)
		block, e := ssh.MarshalPrivateKey(k, "")
		need(e == nil)
		f, e := os.OpenFile(os.Args[2], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		need(e == nil)
		_, e = f.Write(pem.EncodeToMemory(block))
		need(e == nil && f.Sync() == nil && f.Close() == nil)
		signer, e := ssh.NewSignerFromKey(k)
		need(e == nil)
		json.NewEncoder(os.Stdout).Encode(map[string]any{"host_key_created": true, "fingerprint": ssh.FingerprintSHA256(signer.PublicKey())})
		return
	}
	if len(os.Args) != 5 || os.Args[4] != "--allow-disposable-key-and-revocation" {
		fmt.Fprintln(os.Stderr, "arguments: native-config panel-HTTPS-base approved-demo-UUID --allow-disposable-key-and-revocation; Client key on stdin")
		os.Exit(2)
	}
	key, e := bufio.NewReader(io.LimitReader(os.Stdin, 1025)).ReadString('\n')
	need(e == nil || e == io.EOF)
	key = strings.TrimSpace(key)
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "real panel Client key registers ephemeral SSH public key; node authenticates with real remote SFTP API", "public_sftp_tested": false, "actual_subuser_removal_tested": false, "checks": checks, "passed": false}
	failed := false
	fingerprint := ""
	keyName := ""
	owned := false
	dir := ""
	var root *os.Root
	var sshClients []*ssh.Client
	panel := os.Args[2]
	httpClient := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(base, token, method, path string, body any) (int, []byte) {
		var input io.Reader
		if body != nil {
			b, e := json.Marshal(body)
			need(e == nil)
			input = bytes.NewReader(b)
		}
		r, e := http.NewRequest(method, base+path, input)
		need(e == nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json")
		resp, e := httpClient.Do(r)
		need(e == nil)
		defer resp.Body.Close()
		b, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		need(e == nil)
		return resp.StatusCode, b
	}
	removeKey := func() {
		if fingerprint == "" {
			return
		}
		status, _ := call(panel, key, "POST", "/api/client/account/ssh-keys/remove", map[string]string{"fingerprint": fingerprint})
		need(status == 204)
		status, b := call(panel, key, "GET", "/api/client/account/ssh-keys", nil)
		need(status == 200)
		var keys struct {
			Data []struct {
				Attributes struct {
					Fingerprint string `json:"fingerprint"`
				} `json:"attributes"`
			} `json:"data"`
		}
		need(json.Unmarshal(b, &keys) == nil)
		for _, item := range keys.Data {
			need(item.Attributes.Fingerprint != fingerprint)
		}
		fingerprint = ""
		checks["ephemeral_panel_ssh_key_removed"] = true
	}
	defer func() {
		if recover() != nil {
			failed = true
		}
		for _, c := range sshClients {
			c.Close()
		}
		if fingerprint != "" {
			func() {
				defer func() {
					if recover() != nil {
						failed = true
						report["key_cleanup_failed"] = true
					}
				}()
				removeKey()
			}()
		}
		if root != nil {
			if owned {
				for _, name := range []string{"binary", "new", "renamed"} {
					if e := root.Remove(dir + "/" + name); e != nil && !os.IsNotExist(e) {
						failed = true
					}
				}
				if e := root.Remove(dir); e != nil && !os.IsNotExist(e) {
					failed = true
				}
				_, e := root.Lstat(dir)
				checks["owned_fixture_removed"] = os.IsNotExist(e)
				if !checks["owned_fixture_removed"] {
					failed = true
				}
			}
			root.Close()
		}
		report["passed"] = !failed
		report["finished_utc"] = time.Now().UTC().Format(time.RFC3339)
		json.NewEncoder(os.Stdout).Encode(report)
		if failed {
			os.Exit(1)
		}
	}()
	need(strings.HasPrefix(key, "ptlc_") && len(key) < 1024 && panel == "https://panel.vanity.gg")
	b, e := os.ReadFile(os.Args[1])
	need(e == nil)
	var cfg struct {
		Token   string `json:"token"`
		TokenID string `json:"token_id"`
		Data    string `json:"data_directory"`
		SFTP    struct {
			Listen  string `json:"listen"`
			HostKey string `json:"host_key"`
		} `json:"sftp"`
		Profiles map[string]json.RawMessage `json:"profiles"`
	}
	need(json.Unmarshal(b, &cfg) == nil)
	id := os.Args[3]
	need(id == "58e8c7f9-4923-43b3-b431-90b2bf4edf4f" && regexp.MustCompile(`^127\.0\.0\.1:\d+$`).MatchString(cfg.SFTP.Listen))
	_, ok := cfg.Profiles[id]
	need(ok)
	hostPEM, e := os.ReadFile(cfg.SFTP.HostKey)
	need(e == nil)
	hostSigner, e := ssh.ParsePrivateKey(hostPEM)
	need(e == nil)
	report["host_fingerprint"] = ssh.FingerprintSHA256(hostSigner.PublicKey())
	hostPEM = nil
	status, b := call(panel, key, "GET", "/api/client/account", nil)
	need(status == 200)
	var account struct {
		Attributes struct {
			User     string `json:"uuid"`
			Username string `json:"username"`
		} `json:"attributes"`
	}
	need(json.Unmarshal(b, &account) == nil && account.Attributes.Username != "")
	_, private, e := ed25519.GenerateKey(rand.Reader)
	need(e == nil)
	clientSigner, e := ssh.NewSignerFromKey(private)
	need(e == nil)
	keyName = "wings-sftp-test-" + nonce()
	status, b = call(panel, key, "POST", "/api/client/account/ssh-keys", map[string]string{"name": keyName, "public_key": string(ssh.MarshalAuthorizedKey(clientSigner.PublicKey()))})
	need(status == 200 || status == 201)
	var registered struct {
		Attributes struct {
			Name        string `json:"name"`
			Fingerprint string `json:"fingerprint"`
		} `json:"attributes"`
	}
	need(json.Unmarshal(b, &registered) == nil && registered.Attributes.Fingerprint != "" && registered.Attributes.Name == keyName)
	fingerprint = registered.Attributes.Fingerprint
	checks["ephemeral_ssh_key_registered_through_real_client_api"] = true
	status, b = call(panel, cfg.TokenID+"."+cfg.Token, "POST", "/api/remote/sftp/auth", map[string]any{"type": "public_key", "username": account.Attributes.Username + "." + id[:8], "password": strings.TrimSpace(string(ssh.MarshalAuthorizedKey(clientSigner.PublicKey()))), "ip": "127.0.0.1", "session_id": []byte("probe-preflight"), "client_version": []byte("SSH-2.0-WingsWindowsProbe")})
	need(status == 200)
	var remoteIdentity struct {
		User   string `json:"user"`
		Server string `json:"server"`
	}
	need(json.Unmarshal(b, &remoteIdentity) == nil && remoteIdentity.Server == id && regexp.MustCompile(`^[0-9a-f-]{36}$`).MatchString(remoteIdentity.User))
	account.Attributes.User = remoteIdentity.User
	checks["real_remote_sftp_auth_contract_resolves_approved_server"] = true
	root, e = os.OpenRoot(filepath.Join(cfg.Data, id))
	need(e == nil)
	dir = "sftp-probe-" + nonce()
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	data := make([]byte, 9<<20)
	_, e = rand.Read(data)
	need(e == nil)
	f, e := root.OpenFile(dir+"/binary", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	need(e == nil)
	_, e = f.Write(data)
	need(e == nil && f.Close() == nil)
	sshConfig := func(signer ssh.Signer) *ssh.ClientConfig {
		return &ssh.ClientConfig{User: account.Attributes.Username + "." + id[:8], Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(hostSigner.PublicKey()), Timeout: 8 * time.Second}
	}
	connect := func() (*ssh.Client, *sftp.Client) {
		c, e := ssh.Dial("tcp", cfg.SFTP.Listen, sshConfig(clientSigner))
		need(e == nil)
		sshClients = append(sshClients, c)
		client, e := sftp.NewClient(c)
		need(e == nil)
		return c, client
	}
	c, client := connect()
	checks["real_panel_public_key_authentication_succeeds"] = true
	items, e := client.ReadDir("/")
	need(e == nil)
	found := false
	for _, item := range items {
		if item.Name() == "install.marker" {
			found = true
		}
	}
	need(found)
	checks["sftp_root_browsing_succeeds"] = true
	marker, e := client.Open("install.marker")
	need(e == nil)
	markerBytes, e := io.ReadAll(io.LimitReader(marker, 128))
	marker.Close()
	need(e == nil && string(markerBytes) == "installed")
	checks["existing_marker_read_without_mutation"] = true
	file, e := client.Open(dir + "/binary")
	need(e == nil)
	got, e := io.ReadAll(io.LimitReader(file, (9<<20)+1))
	file.Close()
	need(e == nil && sha256.Sum256(got) == sha256.Sum256(data))
	checks["nine_mib_sftp_binary_integrity"] = true
	if created, e := client.Create(dir + "/new"); e == nil {
		created.Close()
		panic("write accepted")
	}
	checks["sftp_upload_refused"] = true
	need(client.Remove(dir+"/binary") != nil && client.Rename(dir+"/binary", dir+"/renamed") != nil && client.Chmod(dir+"/binary", 0777) != nil && client.Mkdir(dir+"/new") != nil)
	checks["sftp_mutations_refused"] = true
	for _, name := range []string{"../private/config.json", `C:\WingsWindows\private\config.json`, "install.marker:stream"} {
		if f, e := client.Open(name); e == nil {
			f.Close()
			panic("unsafe path accepted")
		}
	}
	checks["unsafe_windows_and_parent_paths_refused"] = true
	if _, _, e := c.OpenChannel("direct-tcpip", nil); e == nil {
		panic("forwarding accepted")
	}
	checks["ssh_forwarding_refused"] = true
	client.Close()
	c.Close()
	// A separate SSH connection proves shell/exec cannot use this service.
	control, e := ssh.Dial("tcp", cfg.SFTP.Listen, sshConfig(clientSigner))
	need(e == nil)
	sshClients = append(sshClients, control)
	session, e := control.NewSession()
	need(e == nil)
	need(session.Run("echo forbidden") != nil)
	session.Close()
	control.Close()
	checks["ssh_exec_refused"] = true
	_, badKey, _ := ed25519.GenerateKey(rand.Reader)
	badSigner, _ := ssh.NewSignerFromKey(badKey)
	if bad, e := ssh.Dial("tcp", cfg.SFTP.Listen, sshConfig(badSigner)); e == nil {
		bad.Close()
		panic("unregistered key accepted")
	}
	checks["unregistered_public_key_refused"] = true
	_, revoked := connect()
	open, e := revoked.Open(dir + "/binary")
	need(e == nil)
	status, _ = call("http://127.0.0.1:8080", cfg.Token, "POST", "/api/deauthorize-user", map[string]any{"user": account.Attributes.User, "servers": []string{id}})
	need(status == 204)
	_, e = open.ReadAt(make([]byte, 32), 0)
	need(e != nil)
	open.Close()
	revoked.Close()
	checks["scoped_node_user_revocation_stops_open_sftp_read"] = true
	time.Sleep(1100 * time.Millisecond)
	_, fresh := connect()
	freshFile, e := fresh.Open(dir + "/binary")
	need(e == nil)
	freshFile.Close()
	fresh.Close()
	checks["fresh_panel_authorized_session_after_revocation_succeeds"] = true
	removeKey()
	if removed, e := ssh.Dial("tcp", cfg.SFTP.Listen, sshConfig(clientSigner)); e == nil {
		removed.Close()
		panic("removed key accepted")
	}
	checks["removed_panel_ssh_key_cannot_authenticate_again"] = true
	current, e := root.ReadFile(dir + "/binary")
	need(e == nil && sha256.Sum256(current) == sha256.Sum256(data))
	checks["fixture_data_preserved_after_refused_writes"] = true
	report["binary_bytes"] = len(data)
}
