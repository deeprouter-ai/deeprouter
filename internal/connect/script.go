package connect

import (
	"embed"
	"fmt"
	"strings"
)

// The scripts live in templates/ as real, runnable files rather than as string
// literals built up in Go. Three reasons, in order of how much they matter:
// the key page links a "read this before you run it" URL straight at that
// directory (PRD §5.3), so what a cautious user reads has to be the same text
// that runs; `sh -n` and the PowerShell parser can check them; and a reviewer
// can follow a shell script but not a thousand WriteString calls.
//
//go:embed templates/setup.sh templates/setup.ps1 templates/uninstall.sh templates/uninstall.ps1
var templates embed.FS

// injectMarker is the one line each template reserves for the server. It is a
// comment in both languages, so the templates stay syntactically valid and can
// be checked by their own parsers before anything is injected.
const injectMarker = "# @@DEEPROUTER_INJECT@@"

// Platform is which shell will actually execute the script.
//
// It is decided from the request's User-Agent rather than from anything the
// page said, because the User-Agent is the one thing that describes the
// interpreter about to run this text. A command copied off a Windows browser
// and pasted on a Mac still gets a script its shell can read.
type Platform string

const (
	PlatformPOSIX      Platform = "posix"
	PlatformPowerShell Platform = "powershell"
)

// PlatformFromUserAgent identifies PowerShell, and treats everything else as a
// POSIX shell.
//
// Measured (2026-08-27): `curl` sends "curl/8.21.0"; PowerShell 5.1's
// Invoke-RestMethod sends
// "Mozilla/5.0 (Windows NT; Windows NT 10.0; zh-CN) WindowsPowerShell/5.1.x",
// and PowerShell 7 sends "… PowerShell/7.x". Matching "powershell" covers both
// spellings. Defaulting the unknown case to POSIX is the safe direction: sh is
// what curl users get, and a Windows user reaching here without the marker sees
// a script that fails to parse rather than one that half-runs.
func PlatformFromUserAgent(ua string) Platform {
	if strings.Contains(strings.ToLower(ua), "powershell") {
		return PlatformPowerShell
	}
	return PlatformPOSIX
}

// RenderScript builds the setup script for the shell that is about to run it.
//
// 🔴 The API key and the base URL are injected here and nowhere else. In
// particular there is no default base URL anywhere in the template: more than
// one independent deployment exists, with separate databases and keys that do
// not work across them, and a wrong address fails as a string of 401s that say
// nothing about the address (PRD §0.1 F2). Whichever deployment issued the
// token is the one the script must point at, so the value comes from that
// instance's own server_address.
//
// 🔴 It has to be per-shell. A POSIX script piped into `iex` does not merely
// fail to configure anything — PowerShell reports each unparsable line back to
// the terminal, and the line holding the key is one of them, so the key lands
// in red text on screen. Measured on 2026-08-27 before this split existed.
func RenderScript(platform Platform, baseURL, apiKey string, tools []string) string {
	baseURL = normalizeBaseURL(baseURL)
	if platform == PlatformPowerShell {
		return inject("templates/setup.ps1", powerShellValues(baseURL, apiKey, tools))
	}
	return inject("templates/setup.sh", posixValues(baseURL, apiKey, tools))
}

// RenderUninstall builds the undo script.
//
// It carries no key and no base URL — everything it undoes was recorded on the
// user's own machine by the setup run. That is what lets it live at a fixed
// address with no token: there is nothing here worth stealing, and somebody
// who wants to undo their setup should never be told their command expired.
// RenderDeadTokenScript is what a spent, expired, or orphaned token redeems
// to. It must be a RUNNING script, not a comment block: the transport is
// `curl -fsSL | sh` or `irm | iex`, where comments produce no output at all.
// And it must travel with HTTP 200 - curl's -f discards the body of any
// error status. Both were measured live: the user saw nothing (PRD 6).
func RenderDeadTokenScript(platform Platform, lines ...string) string {
	var b strings.Builder
	if platform == PlatformPowerShell {
		for _, l := range lines {
			b.WriteString("Write-Host '" + l + "'" + "\r\n")
		}
		b.WriteString("exit 1" + "\r\n")
		return b.String()
	}
	b.WriteString("#!/bin/sh" + "\n")
	for _, l := range lines {
		b.WriteString("echo '" + l + "'" + "\n")
	}
	b.WriteString("exit 1" + "\n")
	return b.String()
}
func RenderUninstall(platform Platform) string {
	if platform == PlatformPowerShell {
		return inject("templates/uninstall.ps1", "")
	}
	return inject("templates/uninstall.sh", "")
}

// normalizeBaseURL drops a trailing slash so the templates can append paths
// without producing "//v1", which some deployments answer with a redirect that
// curl does not follow by default.
func normalizeBaseURL(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}

func posixValues(baseURL, apiKey string, tools []string) string {
	return fmt.Sprintf("DR_BASE_URL=%s\nDR_API_KEY=%s\nDR_TOOLS=%s",
		shellQuote(baseURL), shellQuote(apiKey), shellQuote(strings.Join(tools, " ")))
}

func powerShellValues(baseURL, apiKey string, tools []string) string {
	quoted := make([]string, 0, len(tools))
	for _, t := range tools {
		quoted = append(quoted, psQuote(t))
	}
	return fmt.Sprintf("$DrBaseUrl = %s\n$DrApiKey  = %s\n$DrToolIds = @(%s)",
		psQuote(baseURL), psQuote(apiKey), strings.Join(quoted, ", "))
}

// inject swaps the marker line for the generated assignments. A template that
// somehow lost its marker would otherwise render as a script with no key and no
// address, which fails in a confusing way; refusing outright is clearer.
func inject(name, values string) string {
	body, err := templates.ReadFile(name)
	if err != nil {
		return "# The setup script is unavailable on this server.\n"
	}
	text := string(body)
	if values == "" {
		return text
	}
	if !strings.Contains(text, injectMarker) {
		return "# The setup script is unavailable on this server.\n"
	}
	return strings.Replace(text, injectMarker, values, 1)
}

// shellQuote wraps a value in single quotes for POSIX sh, escaping any single
// quote inside it. The key is attacker-influenced only via the database, but a
// value that breaks out of its quoting would turn a config step into command
// execution on the user's machine — cheap to prevent, expensive to miss.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote is the same idea for PowerShell, which escapes a single quote inside
// a single-quoted string by doubling it and expands nothing else.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
