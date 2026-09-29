package connect

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// The signed macOS installer (PRD §11.7). Unlike the .cmd/.sh wrappers this
// file cannot be generated per download: the Developer ID signature freezes
// the package body, and notarization runs on Apple's servers at minute scale —
// so ONE artifact is built and signed per release, and everything that varies
// per download has to travel OUTSIDE the body. The filename is the only thing
// a download leaves writable, so it carries both variables:
//
//	deeprouter-setup-<TOKEN>-<host[_port]>.pkg
//
// The host rides along for the same reason a default base URL is banned
// everywhere else (PRD §0.1 F2): several independent deployments exist, and
// the postinstall must call back to the one that issued the token. The token
// alphabet contains no '-', so the first '-' after the prefix splits the two
// even though hosts may themselves contain one (deep-router.com does). Ports
// are spelled with '_' because ':' does not survive macOS filenames; the
// postinstall maps loopback hosts to http and everything else to https
// (production always terminates TLS at Caddy).

// pkgFileEnv points at the signed artifact on this deployment. Unset means the
// deployment carries no installer and the endpoint stays dark — the build
// pipeline exists in CI, not in this binary.
const pkgFileEnv = "DEEPROUTER_PKG_FILE"

// installerFilePkgBare is the artifact's unpersonalized name. A request for it
// is answered with the tokenized name in Content-Disposition; a client that
// ignores that header saves a file the postinstall will refuse with a
// readable "download a fresh copy" report rather than half-run.
const installerFilePkgBare = "deeprouter-setup.pkg"

// PkgFileName builds the per-download filename for one token, from this
// instance's own server address — never from a constant.
func PkgFileName(token, serverAddress string) (string, error) {
	u, err := url.Parse(normalizeBaseURL(serverAddress))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("connect: server address %q has no usable host", serverAddress)
	}
	host := strings.ReplaceAll(u.Host, ":", "_")
	return "deeprouter-setup-" + token + "-" + host + ".pkg", nil
}

// pkgArtifact reads the signed artifact, if this deployment has one.
func pkgArtifact() ([]byte, error) {
	path := os.Getenv(pkgFileEnv)
	if path == "" {
		return nil, fmt.Errorf("connect: %s is not configured", pkgFileEnv)
	}
	return os.ReadFile(path)
}
