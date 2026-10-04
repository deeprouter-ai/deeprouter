# `macos-pkg` — the signed macOS installer

Assets for the double-click `.pkg` route of one-click setup (PRD §11.7, card
P8). Everything here is **built on macOS** — `pkgbuild`/`productbuild` are
Xcode tools — by `.github/workflows/macos-pkg.yml`; the gateway never builds
it, it only serves the finished artifact.

## The one idea everything follows from

The Developer ID signature freezes the package body, and notarization runs on
Apple's servers at minute scale — so **one artifact is signed per release**,
and everything personal travels in the **filename**:

```
deeprouter-setup-<TOKEN>-<host[_port]>.pkg
```

- The gateway serves the same signed bytes under a per-download name
  (`GET /d/:token/…`, `internal/connect/installer_pkg.go`), consuming nothing —
  the token must survive until the package runs.
- `scripts/postinstall` parses its own filename (`$PACKAGE_PATH`), fetches
  `/i/<token>` from the issuing host — the same endpoint the one-line command
  uses — and runs it. Split on the FIRST `-` after the prefix: the token
  alphabet has no `-`, hosts may (`deep-router.com`). `_` spells `:` (ports);
  loopback → http, everything else https. Browser rename (`xxx (1).pkg`)
  is tolerated.
- The report lands in `~/Library/Logs/deeprouter-setup.log`; the Installer's
  conclusion pane (`resources/conclusion.html`) says so, because a GUI install
  has no terminal to read.

## Building

```sh
sh build.sh            # → dist/deeprouter-setup-unsigned.pkg  (no credentials)
DR_SIGN_IDENTITY="Developer ID Installer: …" sh build.sh   # + sign
# + DR_NOTARY_KEY_FILE / DR_NOTARY_KEY_ID / DR_NOTARY_ISSUER → notarize + staple
```

Unsigned output is a test artifact: Gatekeeper blocks it on double-click
(right-click → Open to test anyway). The signed+notarized+stapled one is the
shippable thing.

## Deploying

The gateway reads the artifact from the path in `DEEPROUTER_PKG_FILE`. Unset →
the pkg endpoint answers 404 and the feature stays dark. Wiring the CI
artifact onto the production box (and switching the key page's macOS button
from `.sh` to `.pkg`) is the last step of card P8 and updates the deploy docs
with it.

## Known real-machine risk

User-domain install (`enable_currentUserHome`) of a payload-free package is
what avoids the admin-password prompt — and Installer's support for it has
historically wobbled between macOS releases. That is exactly the step the
boss verifies on a real Mac; the recorded fallback is a minimal payload into
`~/.deeprouter/` (see the P8 card).
