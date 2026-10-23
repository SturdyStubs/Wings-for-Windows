# Alpha 42 live tunnel recovery, files and permissions

Recorded October 23, 2026 (America/Phoenix). UTC timestamps are retained in the [sanitized results](server2022-alpha42-live-operations-results.json). This follows the [first live demo deployment](server2022-alpha42-live-demo.md). The runtime still uses that immutable alpha 42 package; the separately deployed tunnel supervisor is bound by its SHA256 in the results.

## Browser console report

The operator pasted actual panel console output showing starting/running events, `SERVER_READY`, child output and `ECHO:hello`. This is operator-supplied evidence of browser console streaming and command delivery, not an agent browser inspection, disconnect/reconnection or subuser-revocation test. Stop worked in earlier direct API checks; browser-driven stop was not independently confirmed.

## Tunnel persistence and recovery

A startup-triggered Windows task runs the new pinned-key SSH supervisor as SYSTEM. Its separate ProgramData directory permits only Administrators/SYSTEM and contains the copied tunnel key, the already accepted VPS public host-key entry and the supervisor. SSH uses batch mode, no inherited SSH configuration or agent, strict known-host checking, loopback-only reverse forwarding, 15-second server-alive probes and a 10-second reconnect interval. The task has no execution time limit, ignores parallel starts and has bounded task-failure restart settings. The manual SSH process was replaced without changing the VPS configuration.

Authenticated public HTTPS returned 200 through the task. A controlled forced termination of its exact SSH child produced a new child PID and restored authenticated HTTPS within the bounded probe window. The task remains Running. Daemon service startup is now Automatic. No host reboot, suspended VM, physical network loss, VPS outage, changed host key or repeated supervisor-failure test was performed. Configured boot startup is not verified boot recovery. The existing VPS key restrictions still govern the remote forward.

The initial child-termination command required `-Force` because Windows PowerShell otherwise prompts when terminating a SYSTEM-owned process in a noninteractive session; it did not terminate that child. The corrected controlled check passed.

## Live file checks

Nine groups pass via authenticated public HTTPS against the real panel-assigned demo: anonymous mutation rejection, nested directory creation, text write/read, directory listing, actual restricted-game read/write, rename/move plus game access, numbered copies plus game access, traversal rejection and OS inspection of the demo process owner. The demo command probes run inside the actual daemon-launched game process. Their files remain under `file-manager` for panel inspection. These are API checks; panel file-editor/browser controls have not been exercised by the agent. No egg denylist or cross-server probe was added to this real demo.

## Account permission and credential recovery checks

A temporary password-backed task under the same dedicated game account confirms read denial for the private node config, actual tunnel identity and actual machine-protected operator credential, denial of code-file write access, and successful own-data write/read. It does not use the game's restricting token; the separate process probes above exercise the launched game. This is not comprehensive isolation certification. The probe task and temporary shared-code script were removed and their absence independently confirmed.

The initial user-scope DPAPI credential exports could not be decrypted from a new key-authenticated SSH session (`Key not valid for use in specified state`). No secret was displayed. Both dedicated account passwords were deliberately rotated, the SCM logon credential updated and the daemon-context game credential replaced through the existing helper. New recovery records use machine-scope DPAPI inside the Administrators/SYSTEM-only operator directory. Both records were decrypted and validated from a subsequent SSH session without printing credentials; obsolete user-scope exports were removed. The game-account task successfully used the new game recovery record. Machine DPAPI relies on the directory ACL for user separation; it is not protection from local Administrators/SYSTEM. The earlier user-scope recovery claim is superseded by this correction.

Final inspection: authenticated public HTTPS 200, service Running, tunnel task Running, demo running, no temporary permission-probe task or script. Accounts, service, credentials, tunnel and demo files are intentionally retained for use. Automatic configuration updates, panel file UI, subuser revocation, real games, player routing, SFTP and new reboot validation remain open. No milestone is closed.
