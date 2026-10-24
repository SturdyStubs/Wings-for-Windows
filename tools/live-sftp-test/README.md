# Opt-in real-panel SFTP probe

Build with `go build -trimpath -o dist/live-sftp-test.exe ./tools/live-sftp-test` and run on the dedicated disposable Windows demo node. Normal runs require four arguments: native configuration path, `https://panel.vanity.gg`, approved demo UUID `58e8c7f9-4923-43b3-b431-90b2bf4edf4f`, and `--allow-disposable-key-and-revocation`. Supply the temporary Client API key through stdin, never arguments or a saved script.

The operator-only setup mode `live-sftp-test.exe --generate-host-key <new-absolute-path>` exclusively creates an unencrypted Ed25519 host key and prints only its public fingerprint. Protect its ACL before enabling the daemon; it never overwrites an existing key.

The live probe registers one ephemeral public key on the Client account, resolves its user/server through the node-authenticated remote API, and tests actual loopback SSH/SFTP authentication, listing, 9 MiB integrity, mutation/path/exec/forwarding refusal and scoped revocation. It removes its own panel key and exact owned fixture; unexpected directory contents are retained and fail cleanup. Revocation also invalidates that user's existing demo console tokens; use a disposable server. It does not create a panel account or send account email. It never reports API keys, private keys or native configuration.

Password authentication and actual panel subuser removal are not proved by this tool. The public node HTTP route and public SFTP are distinct; loopback success does not prove internet SFTP connectivity. A failed probe must be investigated, including key/fixture cleanup flags, before claiming acceptance.


Alpha 54 adds an alternative explicit flag `--allow-disposable-key-and-uploads`, which requires the native `sftp.allow_uploads` opt-in. It adds actual 9 MiB out-of-order upload/readback, no final name before CLOSE, empty files, disconnect cleanup, refusal of existing destinations and revoked-upload nonpublication. Both modes retain ephemeral key removal and exact owned-file cleanup. Use the original flag only while the node remains read-only.


Alpha 55 adds `--allow-disposable-key-and-management` for the dedicated node with both upload and management opt-ins. It adds real mkdir, file/nonempty-directory rename, collision preservation, typed/nonempty-delete refusal, file removal, empty-directory removal and revoked-command refusal. Only the exact known nested fixture files and empty directories are cleaned; unexpected contents fail cleanup and are retained.
