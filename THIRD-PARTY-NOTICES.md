# Third-party notices

This experimental implementation follows the Pterodactyl Wings protocol. The upstream MIT notice is retained in `LICENSE`:

- [Pterodactyl Wings](https://github.com/pterodactyl/wings), reference commit `747bc43`, MIT license, copyright Dane Everitt and contributors.

Direct Go dependencies are pinned in `go.mod` and checksummed in `go.sum`:

- [golang-jwt/jwt v5.3.0](https://github.com/golang-jwt/jwt/tree/v5.3.0), MIT.
- [gorilla/websocket v1.5.3](https://github.com/gorilla/websocket/tree/v1.5.3), BSD-2-Clause.
- [golang.org/x/sys v0.49.0](https://cs.opensource.google/go/x/sys/+/refs/tags/v0.49.0:), BSD-3-Clause.

The binary archive includes the complete dependency notices under `licenses/`. Go itself uses a BSD-style license; its license is included there as well. This project is not endorsed by Pterodactyl or Microsoft.
