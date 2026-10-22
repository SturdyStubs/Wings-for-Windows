# Source RCON command adapter

Alpha 30 adds optional Source TCP RCON commands and command-based graceful stops. Existing profiles continue using stdin. This does not add Rust WebRCON, BattlEye, UDP RCON, Windows control events or log-file tailing, or certify a particular real game.

## Profile

```json
"source_rcon": {
  "address": "127.0.0.1:27015",
  "password_environment": "WINGS_RCON_MY_SERVER",
  "timeout_seconds": 3
},
"stop_command": "quit"
```

Choose the stop command documented by your game; `quit` is illustrative. The local operator approves the endpoint and secret reference. Panel environment values cannot change them. Only literal `127.0.0.1` and `[::1]` with ports 1..65535 are accepted, without DNS. Exact endpoints must be unique between profiles. Configure the game's matching password and bind its RCON listener to loopback. This protocol is unencrypted. Loopback restricts network reach but does not attest that the listener belongs to the current game; other trusted local processes can bind that port.

The password comes from the **daemon process environment**, with a name beginning `WINGS_RCON_`. It is not read from the panel or inherited by the game. Do not add that name to `allowed_environment`. Password values are absent from node profiles, API status and client errors. Exact matches in assembled replies are replaced with `[REDACTED]` before console publication, including matches spanning response packets. This does not redact arbitrary game stdout, command text, encoded secrets or other panel environment values.

Supply the daemon environment through secure launch/deployment tooling. SCM deployments must arrange that environment and restart the service after changes. This release adds no secret provisioning command. System-wide environment variables and default service registry values may be readable by other local users; an environment variable is not a credential vault. Check access before storing private passwords. Same-account games cannot be isolated from their daemon identity. Existing private/trusted-node restrictions remain.

Readiness still comes from stdout/egg matchers. A game with no useful stdout needs a future log-tail/readiness adapter. Configuration validation and preflight do not authenticate or make network requests. An unavailable password fails command delivery, rather than preventing the game from starting.

## Delivery, bounds and failures

Each command uses a fresh authenticated connection and is sent once. Existing REST/WebSocket authorization applies. Concurrent commands receive a busy error without sending. There is no reconnect, retry or stdin fallback. Correct authentication failures before retrying; games can ban repeated failed clients.

Commands/passwords allow at most 4086 UTF-8 bytes and no NUL/CR/LF; commands must be nonempty. Before allocating, the client limits each packet size field to 10..4096 bytes, including ID/type/padding. It requires a matching successful authentication response, accepting an optional preceding empty response-value packet.

Completion requires SRCDS's empty response-value mirror: after the command, a distinct-ID empty `SERVERDATA_RESPONSE_VALUE` marks the end of ordered command replies. Engines without that behavior need another adapter. There is no first-packet-only success or idle-time completion guess. The client bounds response packets to 128, assembled text to 64 KiB and lines to 256. It rejects wrong IDs/types/padding, incomplete/oversized replies, invalid UTF-8/NUL text and missing completion. Partial output is discarded; UTF-8 validation and redaction occur after assembly.

The total dial/auth/write/read deadline defaults to three seconds, configurable from one to ten. Process exit, emergency kill and lifecycle cancellation close the connection. Replies appear as `[RCON] ...` only while the originating process is current; they never drive readiness or native process state.

An error after transmission leaves command execution **unconfirmed**. Do not blindly repeat destructive commands. Stop delivery failure produces a fixed safe log message, then waits for native exit and forces the whole Job Object tree down after the normal stop timeout. Exchange time precedes that grace period. Kill bypasses RCON. A successful reply alone does not prove the game stopped.

## Protocol and verification

The contract follows [Valve's Source RCON description](https://developer.valvesoftware.com/wiki/Source_RCON_Protocol). Framing/authentication were also cross-checked with the [gorcon author's implementation](https://github.com/gorcon/rcon/blob/master/rcon.go); no code or dependency from it was added. Local tests use independent literal wire bytes and a separate harmless demo TCP implementation.

Tests cover split replies/redaction, malformed/oversized/truncated frames, authentication and ID/type mismatch, missing completion, budgets, cancellation, secret non-inheritance, busy rejection, native kill, graceful stops and failed-stop fallback. The service fixture's `-TestSourceRCON` uses the public fixed password of the harmless loopback demo, not a production credential. The clean release passes 619 local cases/subtests, 14 packaged native cases and all 54 combined Server 2022 service checks. See [exact package/source/cleanup evidence](evidence/server2022-alpha30-source-rcon.md). No real game or new reboot certification is claimed.
