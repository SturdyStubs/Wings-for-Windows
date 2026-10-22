# Create a panel record, then approve Windows execution

Alpha 34 adds **opt-in, manual-approval creation** through `POST /api/servers`. It accepts the panel's new server record and keeps it waiting for an operator-owned UUID-specific profile. It does not automatically install software, create an account, grant permissions or start a process.

Enable this in the private node JSON and restart:

```json
"allow_pending_creation": true,
"profiles": {}
```

An empty profile map is valid only with this option. Existing configurations retain disabled creation by default. For a first node, run this zero-profile configuration temporarily in the foreground using the normal `--config` command. Prepare the owned empty layout and trusted TLS first. Create the server record from the panel with **Start on completion disabled**. Once you have its full UUID, stop the foreground daemon, add the approved game profile/account and complete initial managed service setup while data/logs remain empty. See the [complete walkthrough](PANEL-WINDOWS-SETUP.md). Do not run the initial permission installer over an existing populated node.

## Wire contract and validation

The request follows upstream Wings `server/installer.ServerDetails` and the reviewed panel `Services/Daemon/ServerClient::create`:

```json
{"uuid":"11111111-1111-4111-8111-111111111111","start_on_completion":false}
```

Incoming node bearer authentication and the normal 64 KiB JSON limit apply. Creation requires a UUID v4 and a fresh successful lookup in this node's assigned-server list using outgoing node credentials. The POST body is never an executable/profile approval. Arbitrary request paths, startup strings, scripts, credentials and environment fields cannot register execution.

| Result | HTTP |
| --- | --- |
| Assigned UUID accepted for manual approval | 202 |
| Repeat request for the same still-assigned pending UUID | 202, no duplicate or side effect |
| Creation disabled | 501 |
| Invalid UUID or `start_on_completion: true` | 422 |
| Not assigned to this node | 404 |
| UUID already has a local approved profile, or shutdown | 409 |
| Pending capacity (1,024 distinct UUIDs) reached | 429 |
| Panel lookup failure | 502 with generic error, without raw panel details |

Assignment lookup has a ten-second HTTP request bound. A failed/disconnected creation request is not an acknowledgement; inspect the panel before retrying. Pending replay is deliberately idempotent, unlike upstream's blanket duplicate conflict. Approved duplicates still conflict. This route is partial Wings behavior, not a full native replacement for its automatic creation/installation pipeline.

## Waiting and handoff

`GET /api/servers` includes pending records. `GET /api/servers/{uuid}` exposes their remote settings, offline utilization and:

```json
"native_creation": {"stage":"awaiting_profile","execution_approved":false,"installed":false}
```

Pending UUIDs cannot use power, command, install, reinstall or file operations (409). WebSocket upgrades remain unavailable until local approval (404). No server-root directory, console store, account, credential, Job Object, installer event or installation-success callback is created. A panel UI may continue to show **Installing** while it waits; it does not currently have a native “Awaiting Windows approval” status label.

There is no independent on-disk pending queue. Startup discovery reconstructs pending entries from the panel's authoritative assigned list. A later manager discovery removes unassigned pending entries; ordinary GET responses are cached snapshots until discovery/restart. A new create replay always rechecks assignment. No filesystem cleanup is needed for a pending record because no server files were created.

Stop the daemon, add `profiles.FULL_UUID` with a fixed approved executable/arguments and separate game SID/credential target, and provision permissions and credentials using the documented operator workflow. Restart registers it as a normal approved server; its pending marker disappears. Invoke panel **Reinstall** to run the [approved native data/setup recipe](NATIVE-INSTALLERS.md), then start manually. Game binaries and installer prerequisites must already exist. Approval alone does not send an installation-success callback.

For an existing managed installation, adding a game identity still requires administrator-managed code/data ACLs, batch rights and daemon-context credentials. This feature does not safely automate those changes or upgrade the initial permission planner into an add-server tool. Do not reuse another game's account or enable same-account execution to bypass that work.

## Scope

The reviewed panel persists a new record before its daemon create request and removes it when that request fails. Returning 202 addresses that source-level blocker when manual approval is enabled and automatic start is disabled. Tests reproduce that transaction behavior with a mock; an actual panel import/create/reinstall session remains unverified.

Automatic start-on-completion, automatic profile selection, game-account provisioning, protected binary staging/promotion, deletion and durable installer/callback reconciliation remain future work. Do not enable this mode expecting arbitrary Linux eggs to become Windows recipes.
