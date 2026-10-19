# Third-party notices

This experimental implementation follows the Pterodactyl Wings protocol. The upstream MIT notice is retained in `LICENSE`:

- [Pterodactyl Wings](https://github.com/pterodactyl/wings), reference commit `747bc43`, MIT license, copyright Dane Everitt and contributors.

Direct Go dependencies are pinned in `go.mod` and checksummed in `go.sum`:

- [golang-jwt/jwt v5.3.0](https://github.com/golang-jwt/jwt/tree/v5.3.0), MIT.
- [gorilla/websocket v1.5.3](https://github.com/gorilla/websocket/tree/v1.5.3), BSD-2-Clause.
- [golang.org/x/sys v0.49.0](https://cs.opensource.google/go/x/sys/+/refs/tags/v0.49.0:), BSD-3-Clause.
- [gabriel-vasile/mimetype v1.4.16](https://github.com/gabriel-vasile/mimetype/tree/v1.4.16), MIT. The retracted v1.4.15 release is not used.
- [sabhiram/go-gitignore 525f6e181f06](https://github.com/sabhiram/go-gitignore/tree/525f6e181f06), MIT.

- [tailscale/wf 6fbb0a674ee6](https://github.com/tailscale/wf/tree/6fbb0a674ee6), BSD-3-Clause, The Inet.af AUTHORS. Windows Filtering Platform bindings.
- [go4.org/netipx 7e7bdc8411bf](https://github.com/go4org/netipx/tree/7e7bdc8411bf), BSD-3-Clause, The Inet.af AUTHORS; runtime dependency of wf.

Additional indirect tooling requirements from upstream module manifests remain pinned by Go; they are not linked into this daemon.

The binary archive includes the complete dependency notices under `licenses/`. Go itself uses a BSD-style license; its license is included there as well. This project is not endorsed by Pterodactyl or Microsoft.
