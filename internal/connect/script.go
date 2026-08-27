package connect

import (
	"fmt"
	"strings"
)

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
// ⚠️ The body is a placeholder. P1 delivers the token round-trip and the key
// page block; detecting and configuring each tool is P2 ("One-Click CLI Setup -
// P2: The install script for macOS, Linux and Windows"). What is here is a
// real, runnable script for both shells, so the two-step form in PRD §5.4
// works today and so users see something honest rather than a broken download.
//
// 🔴 It has to be per-shell even while it is a placeholder. A POSIX script
// piped into `iex` does not merely fail to configure anything — PowerShell
// reports each unparsable line back to the terminal, and the line holding the
// key is one of them, so the key lands in red text on screen. Measured on
// 2026-08-27 before this split existed.
func RenderScript(platform Platform, baseURL, apiKey string, tools []string) string {
	if platform == PlatformPowerShell {
		return renderPowerShell(baseURL, apiKey, tools)
	}
	return renderPOSIX(baseURL, apiKey, tools)
}

func renderPOSIX(baseURL, apiKey string, tools []string) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# DeepRouter one-click setup\n")
	sb.WriteString("#\n")
	sb.WriteString("# This script was generated for you and contains your API key.\n")
	sb.WriteString("# It is fetched over HTTPS and is not stored anywhere after this run.\n")
	sb.WriteString("set -eu\n\n")

	sb.WriteString(fmt.Sprintf("DEEPROUTER_BASE_URL=%s\n", shellQuote(baseURL)))
	sb.WriteString(fmt.Sprintf("DEEPROUTER_API_KEY=%s\n", shellQuote(apiKey)))
	sb.WriteString(fmt.Sprintf("DEEPROUTER_TOOLS=%s\n\n", shellQuote(strings.Join(tools, " "))))

	sb.WriteString("echo \"DeepRouter\"\n")
	sb.WriteString("echo \"  Server : $DEEPROUTER_BASE_URL\"\n")
	sb.WriteString("echo \"  Tools  : $DEEPROUTER_TOOLS\"\n")
	sb.WriteString("echo \"\"\n")
	sb.WriteString("echo \"Your key was delivered successfully.\"\n")
	sb.WriteString("echo \"Automatic configuration of these tools is not available yet —\"\n")
	sb.WriteString("echo \"until it ships, follow the guides at $DEEPROUTER_BASE_URL/resources\"\n")

	return sb.String()
}

func renderPowerShell(baseURL, apiKey string, tools []string) string {
	var sb strings.Builder

	sb.WriteString("# DeepRouter one-click setup\n")
	sb.WriteString("#\n")
	sb.WriteString("# This script was generated for you and contains your API key.\n")
	sb.WriteString("# It is fetched over HTTPS and is not stored anywhere after this run.\n")
	sb.WriteString("$ErrorActionPreference = 'Stop'\n\n")

	sb.WriteString(fmt.Sprintf("$DeepRouterBaseUrl = %s\n", psQuote(baseURL)))
	sb.WriteString(fmt.Sprintf("$DeepRouterApiKey  = %s\n", psQuote(apiKey)))
	sb.WriteString(fmt.Sprintf("$DeepRouterTools   = %s\n\n", psQuote(strings.Join(tools, " "))))

	sb.WriteString("Write-Host \"DeepRouter\"\n")
	sb.WriteString("Write-Host \"  Server : $DeepRouterBaseUrl\"\n")
	sb.WriteString("Write-Host \"  Tools  : $DeepRouterTools\"\n")
	sb.WriteString("Write-Host \"\"\n")
	sb.WriteString("Write-Host \"Your key was delivered successfully.\"\n")
	sb.WriteString("Write-Host \"Automatic configuration of these tools is not available yet —\"\n")
	sb.WriteString("Write-Host \"until it ships, follow the guides at $DeepRouterBaseUrl/resources\"\n")

	return sb.String()
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
