package connect

import (
	"fmt"
	"strings"
)

// The downloadable installers exist so that configuring a tool needs no terminal
// at all (PRD §11). A window the script opens for itself is fine; what is not
// fine is asking a non-technical user to find "the terminal" and type into it
// (decision D5 — the criterion is whether the user types, not whether a black
// window appears).
//
// Both files are WRAPPERS, and that is the whole design:
//
//   - 🔴 neither may carry the API key (PRD §11.4). What lands in a Downloads
//     folder is a one-time token that is worth nothing once spent or expired, so
//     an old file sitting there — synced, backed up, forwarded — leaks nothing.
//     The script, and only then the key, arrives from /i/<token> at run time.
//   - the setup logic is not duplicated. These files fetch the same script the
//     one-line command fetches; every fix to setup.ps1 / setup.sh reaches the
//     download path for free.
const (
	// InstallerFileWindows and InstallerFileUnix are what each platform's file
	// is called. The name is in the URL as well as in Content-Disposition,
	// because a client that ignores that header still has to save something
	// Windows will run when it is double-clicked — a file saved under the bare
	// token has no extension and clicking it does nothing at all.
	InstallerFileWindows = "deeprouter-setup.cmd"
	InstallerFileUnix    = "deeprouter-setup.sh"
)

// RenderInstaller builds the file behind the key page's download button.
func RenderInstaller(platform Platform, baseURL, token string) string {
	url := normalizeBaseURL(baseURL) + "/i/" + token
	if platform == PlatformPowerShell {
		return windowsInstaller(url)
	}
	return unixInstaller(url)
}

// windowsInstaller wraps the existing PowerShell one-liner in a .cmd file.
//
// 🔴 `pause` belongs HERE, in the wrapper, and not in setup.ps1. Two separate
// requirements hang on that placement:
//
//   - it runs after PowerShell has exited and never looks at an exit code, so
//     the window stays open on failure too. The run that most needs reading is
//     the one that went wrong, and a self-opened window otherwise vanishes the
//     instant the script ends, taking the whole per-tool report with it.
//   - the one-line `irm | iex` flow stays byte-identical. Somebody who already
//     has a terminal open must not suddenly acquire a "press any key" step.
//
// English only, deliberately: a .cmd is read in the console's OEM code page, so
// non-ASCII text in it arrives as mojibake on a default Chinese Windows. The
// report setup.ps1 itself prints is English for the same reason.
func windowsInstaller(url string) string {
	lines := []string{
		"@echo off",
		"REM DeepRouter setup. Double-click this file - there is nothing to type.",
		"REM The link below works once and then expires. Need another? Open your",
		"REM API keys page and download a fresh file.",
		`powershell -NoProfile -ExecutionPolicy Bypass -Command "irm ` + batchEscape(psQuote(url)) + ` | iex"`,
		"echo.",
		"echo Press any key to close this window.",
		"pause >nul",
	}
	// CRLF throughout: a batch file with bare LF line endings is read
	// inconsistently by cmd.exe, and the failure looks like a corrupt script
	// rather than a line-ending problem.
	return strings.Join(lines, "\r\n") + "\r\n"
}

// unixInstaller is the same wrapper for macOS and Linux, run with one `bash`
// line rather than by double-clicking.
//
// `bash <file>` is what makes this free: the file is handed straight to an
// interpreter, so LaunchServices never sees it and Gatekeeper never gets
// involved — no signing, no notarization, nothing to buy. Making a macOS file
// that installs on a double-click is a different artifact (a signed .pkg,
// PRD §11.7) and a different card.
//
// No shebang-and-chmod dance either: the file is never executed directly, so
// the download does not have to arrive with a permission bit set.
func unixInstaller(url string) string {
	return fmt.Sprintf(`#!/bin/sh
# DeepRouter setup for macOS and Linux. Run it with:
#
#     bash ~/Downloads/%s
#
# The link below works once and then expires. Need another? Open your API keys
# page and download a fresh file. Your API key is NOT in this file - the setup
# script fetches it, and that link can only be used one time.
curl -fsSL %s | sh
`, InstallerFileUnix, shellQuote(url))
}

// batchEscape doubles percent signs, which cmd.exe would otherwise read as
// variable references and silently eat. Only the base URL can contain one (a
// percent-encoded path in server_address); the token's alphabet cannot.
func batchEscape(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}
