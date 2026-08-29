#!/bin/sh
# DeepRouter one-click setup (macOS / Linux / WSL / Git Bash).
#
# This is the template. The server injects your base URL, your API key and the
# tools you ticked into the marked block below, then serves the result.
#
# What it does, in order:
#   1. detect which of the tools you ticked are actually installed
#   2. skip any that already have a paid login (switching them costs you twice)
#   3. pick a model by really calling it, not by reading metadata
#   4. write each tool's config - backing it up first, merging, never overwriting
#   5. record what it touched in ~/.deeprouter/installed.json
#   6. verify each protocol with a real request, and report in plain language
#
# It uses only sh, awk, sed and curl - no Node, no Python, nothing to install.
# Undo everything:  curl -fsSL <base>/uninstall | sh

# Deliberately no `set -e`. One tool failing must not abort the others; every
# step records its own outcome and the report at the end tells the whole truth.
set -u

# ---------------------------------------------------------------------------
# @@DEEPROUTER_INJECT@@
# ---------------------------------------------------------------------------

DR_DIR="$HOME/.deeprouter"
DR_MANIFEST="$DR_DIR/installed.json"
DR_STAMP=$(date +%Y%m%d-%H%M%S)
DR_NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
DR_SEP=$(printf '\037')

# ===========================================================================
# Output
# ===========================================================================
# ASCII only, on purpose. Windows PowerShell 5.1 reads a saved .ps1 in the
# system ANSI codepage, so a non-ASCII byte there can break parsing outright
# (PRD 0.1 F6). The two scripts must report identically, so this one stays
# ASCII too rather than drifting apart from its Windows twin.

dr_say()  { printf '%s\n' "$*"; }
dr_ok()   { printf '  [ ok ] %s\n' "$*"; }
dr_skip() { printf '  [skip] %s\n' "$*"; }
dr_none() { printf '  [ -- ] %s\n' "$*"; }
dr_bad()  { printf '  [fail] %s\n' "$*"; }
dr_note() { printf '         %s\n' "$*"; }

# ===========================================================================
# JSON, implemented in awk
# ===========================================================================
# Two of the four tools keep their settings in JSON that may already hold the
# user's own configuration, and the hardest requirement in this whole script is
# that we never damage it (PRD 4.3). Since we may not install jq, we parse and
# re-emit the document ourselves: a file that does not parse is left untouched
# and its tool is skipped, which is exactly the required behaviour for a
# corrupt config.
#
# Paths use \037 as the separator so a key containing a dot or a bracket cannot
# be confused with structure.
#
# Modes:
#   -v mode=edit -v setfile=F   apply "path<TAB>rawjson" lines from F, print result
#   -v mode=flat                print "path<TAB>rawjson" for every scalar
#
# Written without a single apostrophe: the shell embeds it in a '...' string.

DR_AWK_JSON='
function fail(m) { print "ERR " m > "/dev/stderr"; exit 2 }
function rep(k,   i, o) { o = ""; for (i = 0; i < k; i++) o = o "  "; return o }
function skipws() { while (p <= n && index(" \t\r\n", substr(s, p, 1)) > 0) p++ }
function newnode(k,   id) { nid++; kind[nid] = k; nk[nid] = 0; return nid }

function pstring(   st, c) {
  st = p; p++
  while (p <= n) {
    c = substr(s, p, 1)
    if (c == "\\") { p += 2; continue }
    if (c == "\"") { p++; return substr(s, st, p - st) }
    p++
  }
  fail("unterminated string")
}

function unq(q,   b, o, i, c) {
  b = substr(q, 2, length(q) - 2); o = ""
  for (i = 1; i <= length(b); i++) {
    c = substr(b, i, 1)
    if (c != "\\") { o = o c; continue }
    i++; c = substr(b, i, 1)
    if (c == "n") o = o "\n"
    else if (c == "t") o = o "\t"
    else if (c == "r") o = o "\r"
    else o = o c
  }
  return o
}

function qs(v,   o, i, c) {
  o = "\""
  for (i = 1; i <= length(v); i++) {
    c = substr(v, i, 1)
    if (c == "\"") o = o "\\\""
    else if (c == "\\") o = o "\\\\"
    else if (c == "\n") o = o "\\n"
    else if (c == "\t") o = o "\\t"
    else if (c == "\r") o = o "\\r"
    else o = o c
  }
  return o "\""
}

function pliteral(   st, c, id) {
  st = p
  while (p <= n) {
    c = substr(s, p, 1)
    if (index(" \t\r\n,}]", c) > 0) break
    p++
  }
  if (p == st) fail("empty value")
  id = newnode("lit"); raw[id] = substr(s, st, p - st)
  if (raw[id] !~ /^(true|false|null|-?[0-9]+([.][0-9]+)?([eE][-+]?[0-9]+)?)$/) fail("bad literal")
  return id
}

function pobject(   id, kq, c, ch) {
  id = newnode("obj"); p++; skipws()
  if (substr(s, p, 1) == "}") { p++; return id }
  while (1) {
    skipws()
    if (substr(s, p, 1) != "\"") fail("expected key")
    kq = pstring(); skipws()
    if (substr(s, p, 1) != ":") fail("expected colon")
    p++
    ch = pvalue()
    nk[id]++; kid[id, nk[id]] = ch; key[id, nk[id]] = unq(kq)
    skipws(); c = substr(s, p, 1)
    if (c == ",") { p++; continue }
    if (c == "}") { p++; return id }
    fail("expected comma or close brace")
  }
}

function parray(   id, c, ch) {
  id = newnode("arr"); p++; skipws()
  if (substr(s, p, 1) == "]") { p++; return id }
  while (1) {
    ch = pvalue()
    nk[id]++; kid[id, nk[id]] = ch
    skipws(); c = substr(s, p, 1)
    if (c == ",") { p++; continue }
    if (c == "]") { p++; return id }
    fail("expected comma or close bracket")
  }
}

function pvalue(   c, id) {
  skipws()
  if (p > n) fail("unexpected end of input")
  c = substr(s, p, 1)
  if (c == "{") return pobject()
  if (c == "[") return parray()
  if (c == "\"") { id = newnode("str"); raw[id] = pstring(); return id }
  return pliteral()
}

# Parse a standalone JSON document held in txt, restoring the outer state so
# this can be called while the main document is being walked.
function subparse(txt,   ss, sp, sn, id) {
  ss = s; sp = p; sn = n
  s = txt; p = 1; n = length(txt)
  id = pvalue()
  s = ss; p = sp; n = sn
  return id
}

function setp(root, path, val,   parts, m, i, j, cur, found, ch) {
  m = split(path, parts, SEP)
  cur = root
  for (i = 1; i < m; i++) {
    if (kind[cur] != "obj") fail("cannot descend into a non-object")
    found = 0
    for (j = 1; j <= nk[cur]; j++) if (key[cur, j] == parts[i]) { found = j; break }
    if (found > 0 && kind[kid[cur, found]] == "obj") { cur = kid[cur, found]; continue }
    ch = newnode("obj")
    if (found > 0) kid[cur, found] = ch
    else { nk[cur]++; kid[cur, nk[cur]] = ch; key[cur, nk[cur]] = parts[i] }
    cur = ch
  }
  if (kind[cur] != "obj") fail("cannot set a key on a non-object")
  ch = subparse(val)
  for (j = 1; j <= nk[cur]; j++) if (key[cur, j] == parts[m]) { kid[cur, j] = ch; return }
  nk[cur]++; kid[cur, nk[cur]] = ch; key[cur, nk[cur]] = parts[m]
}

function ser(id, ind,   i, o, pad, pad2) {
  if (kind[id] == "str" || kind[id] == "lit") return raw[id]
  pad = rep(ind); pad2 = rep(ind + 1)
  if (kind[id] == "obj") {
    if (nk[id] == 0) return "{}"
    o = "{\n"
    for (i = 1; i <= nk[id]; i++) {
      o = o pad2 qs(key[id, i]) ": " ser(kid[id, i], ind + 1)
      o = o (i < nk[id] ? ",\n" : "\n")
    }
    return o pad "}"
  }
  if (nk[id] == 0) return "[]"
  o = "[\n"
  for (i = 1; i <= nk[id]; i++) {
    o = o pad2 ser(kid[id, i], ind + 1)
    o = o (i < nk[id] ? ",\n" : "\n")
  }
  return o pad "]"
}

function flat(id, path,   i, np) {
  if (kind[id] == "obj") {
    for (i = 1; i <= nk[id]; i++) {
      np = (path == "" ? key[id, i] : path SEP key[id, i])
      flat(kid[id, i], np)
    }
    return
  }
  if (kind[id] == "arr") {
    for (i = 1; i <= nk[id]; i++) {
      np = (path == "" ? (i - 1) : path SEP (i - 1))
      flat(kid[id, i], np)
    }
    return
  }
  print path "\t" raw[id]
}

BEGIN { SEP = sprintf("%c", 31); nid = 0 }
{ doc = doc $0 "\n" }
END {
  s = doc; p = 1; n = length(s)
  root = pvalue()
  skipws()
  if (p <= n) fail("trailing content after the document")
  if (mode == "flat") { flat(root, ""); exit 0 }
  if (setfile != "") {
    while ((getline line < setfile) > 0) {
      t = index(line, "\t")
      if (t > 0) setp(root, substr(line, 1, t - 1), substr(line, t + 1))
    }
    close(setfile)
  }
  print ser(root, 0)
}
'

# dr_json_valid FILE - true when FILE parses as JSON.
dr_json_valid() { awk -v mode=flat "$DR_AWK_JSON" "$1" >/dev/null 2>&1; }

# dr_json_flat FILE - "path<TAB>rawvalue" per scalar, \037 between path segments.
dr_json_flat() { awk -v mode=flat "$DR_AWK_JSON" "$1" 2>/dev/null; }

# dr_json_edit FILE SETFILE - FILE with the overrides applied, on stdout.
dr_json_edit() { awk -v mode=edit -v setfile="$2" "$DR_AWK_JSON" "$1" 2>/dev/null; }

# dr_str VALUE - a JSON string token for VALUE.
dr_str() {
  printf '"%s"' "$(printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g')"
}

# dr_unstr TOKEN - the text inside a JSON string token.
dr_unstr() {
  printf '%s' "$1" | sed -e 's/^"//' -e 's/"$//' -e 's/\\"/"/g' -e 's/\\\\/\\/g'
}

# ===========================================================================
# Tool table
# ===========================================================================
# Three facts per tool, kept together so adding one is a single edit:
#   exe    the command that proves it is installed. PRD 0.1 F9: a config
#          directory does NOT prove it - installing the ChatGPT desktop app
#          creates ~/.codex/config.toml without ever providing `codex`.
#   login  a file whose mere existence means a paid login we must not displace.
#   proto  which protocol it speaks, which decides the model it can be given.

dr_tool_exe() {
  case "$1" in
    claude-code) printf 'claude' ;;
    opencode)    printf 'opencode' ;;
    codex)       printf 'codex' ;;
    gemini-cli)  printf 'gemini' ;;
  esac
}

dr_tool_name() {
  case "$1" in
    claude-code) printf 'Claude Code' ;;
    opencode)    printf 'OpenCode' ;;
    codex)       printf 'Codex CLI' ;;
    gemini-cli)  printf 'Gemini CLI' ;;
  esac
}

dr_tool_login_file() {
  case "$1" in
    claude-code) printf '%s' "$HOME/.claude/.credentials.json" ;;
    codex)       printf '%s' "$HOME/.codex/auth.json" ;;
    gemini-cli)  printf '%s' "$HOME/.gemini/oauth_creds.json" ;;
    opencode)    printf '%s' "$HOME/.local/share/opencode/auth.json" ;;
  esac
}

# Which protocol the tool will actually speak to the gateway.
# Gemini CLI is deliberately "openai": it reaches the gateway over the Gemini
# protocol, but no model in any deployment declares `gemini` support, and the
# gateway converts (PRD 0.1 F19).
dr_tool_proto() {
  case "$1" in
    claude-code) printf 'anthropic' ;;
    *)           printf 'openai' ;;
  esac
}

# Probe budget per tool. PRD 0.1 F20: the gateway reserves price x max_tokens
# up front, so probing with a tiny max_tokens proves only that auth works - the
# user still gets a 403 on their first real sentence. These are the low end of
# what each tool actually sends.
dr_tool_max_tokens() {
  case "$1" in
    claude-code) printf '8192' ;;
    *)           printf '4096' ;;
  esac
}

DR_ALL_TOOLS="claude-code opencode codex gemini-cli"

# ===========================================================================
# Arguments
# ===========================================================================
# `curl | sh` gives the script no terminal on stdin, so it can never ask a
# question (PRD 4.2.2). The default has to be safe on its own and every choice
# arrives as a flag:  curl -fsSL <url> | sh -s -- --force claude-code

DR_ONLY=""
DR_FORCE=""
DR_HELP=0

while [ $# -gt 0 ]; do
  case "$1" in
    --only)  DR_ONLY=$(printf '%s' "${2:-}" | tr ',' ' '); shift 2 || break ;;
    --force) DR_FORCE=$(printf '%s' "${2:-}" | tr ',' ' '); shift 2 || break ;;
    --all)   DR_FORCE="$DR_ALL_TOOLS"; shift ;;
    -h|--help) DR_HELP=1; shift ;;
    *) shift ;;
  esac
done

if [ "$DR_HELP" = "1" ]; then
  dr_say "DeepRouter one-click setup"
  dr_say ""
  dr_say "  --only  a,b   configure only these tools"
  dr_say "  --force a,b   configure them even if they already have a paid login"
  dr_say "  --all         treat every installed tool as --force"
  dr_say ""
  dr_say "  tools: $DR_ALL_TOOLS"
  exit 0
fi

dr_in_list() {
  for _dr_i in $2; do [ "$_dr_i" = "$1" ] && return 0; done
  return 1
}

# ===========================================================================
# HTTP
# ===========================================================================

DR_TMP=$(mktemp -d 2>/dev/null || printf '%s' "${TMPDIR:-/tmp}/deeprouter-$$")
mkdir -p "$DR_TMP" 2>/dev/null
# The key is written into files under here; leaving them behind would undo the
# point of keeping it out of the URL.
trap 'rm -rf "$DR_TMP"' EXIT INT TERM

if ! command -v curl >/dev/null 2>&1; then
  dr_say "DeepRouter setup needs curl, and it is not on this system."
  dr_say "Install curl and run the command again."
  exit 1
fi

# dr_http METHOD URL [BODY] - body to $DR_TMP/body, status code on stdout.
# 000 means the request never completed (no network, DNS, TLS).
dr_http() {
  _dr_m=$1; _dr_u=$2; _dr_b=${3:-}
  if [ -n "$_dr_b" ]; then
    printf '%s' "$_dr_b" > "$DR_TMP/req"
    _dr_c=$(curl -sS -X "$_dr_m" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $DR_API_KEY" \
      -H "x-api-key: $DR_API_KEY" \
      -H "anthropic-version: 2023-06-01" \
      --data-binary "@$DR_TMP/req" \
      -o "$DR_TMP/body" -w '%{http_code}' "$_dr_u" 2>/dev/null)
  else
    _dr_c=$(curl -sS \
      -H "Authorization: Bearer $DR_API_KEY" \
      -H "x-api-key: $DR_API_KEY" \
      -o "$DR_TMP/body" -w '%{http_code}' "$_dr_u" 2>/dev/null)
  fi
  # On failure curl still prints its -w '000', and the old `|| printf '000'`
  # then appended a second one - "000000" matched no classifier case, so
  # the network branch was dead code. Normalize instead of appending.
  if [ -z "$_dr_c" ]; then printf '000'; else printf '%s' "$_dr_c"; fi
}

# The gateway answers in Chinese and puts the two numbers a user needs inside
# the sentence. We pull the numbers out rather than echoing the sentence: this
# script is ASCII (see the output section), and PRD 6 asks for those figures,
# not for that wording.
dr_err_message() {
  [ -f "$DR_TMP/body" ] || return 0
  tr -d '\n' < "$DR_TMP/body" |
    sed -n 's/.*"message"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

# Classify a failed probe. PRD 4.3 needs these apart: all three arrive as 4xx
# on the same deployment, and only one of them is worth telling the user about.
#   whitelist  the token may not use this model     -> try the next candidate
#   missing    listed but not actually there        -> try the next candidate
#   funds      priced out for this account          -> try the next, cheaper one
#   auth       the key itself is wrong              -> stop, nothing will work
#   busy       upstream is loaded                   -> configuration is fine
dr_classify() {
  _dr_code=$1; _dr_msg=$(dr_err_message)
  case "$_dr_code" in
    200) printf 'ok'; return ;;
    401) printf 'auth'; return ;;
    402) printf 'funds'; return ;;
    503|529) printf 'busy'; return ;;
    000) printf 'network'; return ;;
  esac
  case "$_dr_msg" in
    *"无权"*|*"not allowed"*|*"no permission"*) printf 'whitelist'; return ;;
    *"预扣费"*|*"insufficient"*|*"quota"*|*"额度"*) printf 'funds'; return ;;
    *"does not exist"*|*"not found"*|*"model_not_found"*) printf 'missing'; return ;;
  esac
  case "$_dr_code" in
    403) printf 'whitelist' ;;
    404) printf 'missing' ;;
    *)   printf 'other' ;;
  esac
}

# The two figures out of a reserve failure, as "have<TAB>need". PRD 6 wants
# them passed through untouched.
dr_funds_figures() {
  # The currency symbol depends on the gateway's display setting (fullwidth
  # dollar by default; yen or a custom symbol are possible) - so key on no
  # symbol at all: strip the digit-filled "(request id: ...)" tail, then
  # take the last two numbers. Found live: the ASCII-$ match never fired.
  dr_err_message | sed -e 's/(request id[^)]*)//' |
    grep -o '[0-9][0-9.]*' | sed 's/\.$//' |
    awk '{a[NR]=$0} END { if (NR >= 2) printf "%s\t%s", a[NR-1], a[NR] }'
}

# ===========================================================================
# Step 1 - which tools are we allowed to touch
# ===========================================================================

dr_say ""
dr_say "DeepRouter one-click setup"
dr_say ""
dr_say "  Server : $DR_BASE_URL"
dr_say ""
dr_say "  Checking which tools are installed..."

DR_SELECTED=""
for _dr_t in $DR_TOOLS; do
  if [ -n "$DR_ONLY" ] && ! dr_in_list "$_dr_t" "$DR_ONLY"; then continue; fi
  DR_SELECTED="$DR_SELECTED $_dr_t"
done

DR_TARGETS=""       # installed, allowed, to be configured
DR_SKIPPED_LOGIN=""  # installed but already signed in
DR_MISSING=""        # ticked but not installed

for _dr_t in $DR_SELECTED; do
  _dr_name=$(dr_tool_name "$_dr_t")
  _dr_exe=$(dr_tool_exe "$_dr_t")

  if ! command -v "$_dr_exe" >/dev/null 2>&1; then
    DR_MISSING="$DR_MISSING $_dr_t"
    # Naming the directory matters: it is the difference between "you never
    # installed this" and "you installed something that looks like it".
    if [ -d "$HOME/.$_dr_t" ] || [ -d "$HOME/.codex" -a "$_dr_t" = "codex" ] ||
       [ -d "$HOME/.gemini" -a "$_dr_t" = "gemini-cli" ]; then
      dr_none "$_dr_name  config directory found, but no \`$_dr_exe\` command - skipped"
    else
      dr_none "$_dr_name  not installed - skipped"
    fi
    continue
  fi

  _dr_login=$(dr_tool_login_file "$_dr_t")
  if [ -n "$_dr_login" ] && [ -f "$_dr_login" ] && ! dr_in_list "$_dr_t" "$DR_FORCE"; then
    DR_SKIPPED_LOGIN="$DR_SKIPPED_LOGIN $_dr_t"
    dr_skip "$_dr_name  already signed in - left alone"
    dr_note "Switching it to DeepRouter would mean paying for both."
    dr_note "To switch anyway: re-run with --force $_dr_t"
    continue
  fi

  DR_TARGETS="$DR_TARGETS $_dr_t"
  dr_ok "$_dr_name"
done

if [ -z "$(printf '%s' "$DR_TARGETS" | tr -d ' ')" ]; then
  dr_say ""
  if [ -n "$(printf '%s' "$DR_SKIPPED_LOGIN" | tr -d ' ')" ]; then
    # Not a failure - but printing "done" here would be a lie, and the user
    # would go looking for a configuration that was never written (PRD 6).
    dr_say "  Nothing was changed. Every tool found is already signed in."
    dr_say "  Re-run with --force <tool> if you want it switched to DeepRouter."
  else
    dr_say "  None of the supported tools were found on this machine."
    dr_say ""
    dr_say "  Supported: Claude Code, OpenCode, Codex CLI, Gemini CLI"
    dr_say "  Install one, then copy a fresh command from your keys page."
  fi
  dr_say ""
  exit 0
fi

# ===========================================================================
# Step 2 - pick a model by calling it
# ===========================================================================
# Metadata cannot answer this (PRD 0.1 F19): /v1/models carries no prices, the
# two model endpoints do not contain each other, and the list includes models
# that cannot hold a conversation at all. So we call candidates in order and
# take the first that really answers.

dr_probe_openai() {
  dr_http POST "$DR_BASE_URL/v1/chat/completions" \
    "{\"model\":$(dr_str "$1"),\"max_tokens\":$2,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}"
}

dr_probe_anthropic() {
  dr_http POST "$DR_BASE_URL/v1/messages" \
    "{\"model\":$(dr_str "$1"),\"max_tokens\":$2,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}"
}

dr_probe() {
  if [ "$1" = "anthropic" ]; then dr_probe_anthropic "$2" "$3"; else dr_probe_openai "$2" "$3"; fi
}

# Models that answer chat requests. A name test is not enough on its own, but
# it is the only signal that catches the dangerous case: gpt-4o-mini-tts
# answers a chat request with a 403 reserve failure, which reads exactly like
# "top up your account" for a model that will never chat (PRD 0.1 F19).
dr_is_chat_model() {
  case "$1" in
    *tts*|*audio*|*whisper*|*image*|*video*|*vision-preview*|*embed*|*rerank*|*moderation*) return 1 ;;
    *) return 0 ;;
  esac
}

# Cheap-first, by name. Prices are genuinely unavailable for some usable models
# (PRD 0.1 F19), so this heuristic is the fallback the PRD asks for.
dr_model_rank() {
  case "$1" in
    *nano*)  printf '1' ;;
    *mini*)  printf '2' ;;
    *flash*) printf '3' ;;
    *haiku*) printf '4' ;;
    *small*) printf '5' ;;
    *turbo*) printf '6' ;;
    *sonnet*) printf '8' ;;
    *opus*)  printf '9' ;;
    *)       printf '7' ;;
  esac
}

dr_load_models() {
  # The marker is a file, not a variable: dr_choose_model runs inside a command
  # substitution, and nothing it assigns survives that subshell.
  [ -f "$DR_TMP/models.done" ] && { [ -s "$DR_TMP/models" ]; return; }
  : > "$DR_TMP/models"
  : > "$DR_TMP/models.done"
  _dr_code=$(dr_http GET "$DR_BASE_URL/v1/models")
  [ "$_dr_code" = "200" ] || return 1
  # Flatten to "data<US>N<US>field<TAB>value" and fold each model into one
  # line: "id<TAB>type,type". Reading it structurally rather than by grepping
  # keeps ids and endpoint types paired even when the order changes.
  dr_json_flat "$DR_TMP/body" | awk -v SEP="$DR_SEP" -F'\t' '
    {
      nf = split($1, a, SEP)
      if (a[1] != "data" || nf < 3) next
      idx = a[2]
      v = $2
      gsub(/^"|"$/, "", v)
      if (a[3] == "id") id[idx] = v
      else if (a[3] == "supported_endpoint_types") types[idx] = types[idx] "," v
      seen[idx] = 1
    }
    END { for (i in seen) if (id[i] != "") print id[i] "\t" types[i] }
  ' > "$DR_TMP/models"
  [ -s "$DR_TMP/models" ]
}

# dr_candidates PROTO - usable model names for that protocol, cheapest first.
dr_candidates() {
  _dr_proto=$1
  dr_load_models || return 1
  while IFS="$(printf '\t')" read -r _dr_id _dr_types; do
    dr_is_chat_model "$_dr_id" || continue
    # An empty types field means the deployment did not say; trying it is
    # better than dropping a model that may well work.
    if [ -n "$_dr_types" ]; then
      case ",$_dr_types," in *",$_dr_proto,"*) ;; *) continue ;; esac
    fi
    printf '%s %s\n' "$(dr_model_rank "$_dr_id")" "$_dr_id"
  done < "$DR_TMP/models" | sort | awk '{ print $2 }'
}

# Findings from the probe travel back through files. dr_choose_model is called
# as `$(dr_choose_model ...)`, and a command substitution is a subshell - every
# assignment made in there is discarded when it exits. Losing these silently
# turned "your key was rejected" and "you need $X" into "no model could answer".
DR_AUTH_FLAG="$DR_TMP/auth_failed"
DR_FUNDS_FILE="$DR_TMP/funds"
DR_NET_FLAG="$DR_TMP/network"
DR_MODEL_anthropic=""
DR_MODEL_openai=""
DR_MODEL_NOTE_anthropic=""
DR_MODEL_NOTE_openai=""

# dr_choose_model PROTO MAXTOK - echo the model that really answered, or
# nothing. Everything it learns on the way is recorded for the report.
dr_choose_model() {
  _dr_proto=$1; _dr_max=$2

  # deeprouter-auto first: it is the only option that gets smart routing, and
  # whether a deployment supports it changes with the deployment, so asking is
  # the only answer that is not a guess (PRD 4.3 step 1).
  _dr_code=$(dr_probe "$_dr_proto" "deeprouter-auto" "$_dr_max")
  case "$(dr_classify "$_dr_code")" in
    ok) printf 'deeprouter-auto'; return 0 ;;
    auth) : > "$DR_AUTH_FLAG"; return 1 ;;
    network) : > "$DR_NET_FLAG"; return 1 ;;
  esac

  dr_candidates "$_dr_proto" > "$DR_TMP/cand.$_dr_proto" 2>/dev/null
  [ -s "$DR_TMP/cand.$_dr_proto" ] || return 1

  while read -r _dr_m; do
    [ -n "$_dr_m" ] || continue
    _dr_code=$(dr_probe "$_dr_proto" "$_dr_m" "$_dr_max")
    case "$(dr_classify "$_dr_code")" in
      ok)
        printf '%s' "$_dr_m"
        return 0 ;;
      auth)
        : > "$DR_AUTH_FLAG"
        return 1 ;;
      network)
        # No wire, so no candidate can do better - stop probing.
        : > "$DR_NET_FLAG"
        return 1 ;;
      funds)
        # Remember the figures but keep going: a cheaper candidate may still
        # be affordable, and only running out of candidates means "top up".
        _dr_f=$(dr_funds_figures)
        [ -n "$_dr_f" ] && printf '%s' "$_dr_f" > "$DR_FUNDS_FILE" ;;
      busy)
        # Reachable and permitted, just loaded. Good enough to configure.
        printf '%s' "$_dr_m"
        return 0 ;;
    esac
  done < "$DR_TMP/cand.$_dr_proto"
  return 1
}

dr_say ""
dr_say "  Choosing a model that really works..."

for _dr_t in $DR_TARGETS; do
  _dr_proto=$(dr_tool_proto "$_dr_t")
  eval "_dr_have=\${DR_MODEL_$_dr_proto}"
  [ -n "$_dr_have" ] && continue
  _dr_max=$(dr_tool_max_tokens "$_dr_t")
  _dr_picked=$(dr_choose_model "$_dr_proto" "$_dr_max")
  if [ -n "$_dr_picked" ]; then
    eval "DR_MODEL_$_dr_proto=\$_dr_picked"
    if [ "$_dr_picked" = "deeprouter-auto" ]; then
      dr_ok "$_dr_proto: $_dr_picked (smart routing)"
    else
      dr_ok "$_dr_proto: $_dr_picked"
    fi
  else
    if [ -f "$DR_NET_FLAG" ]; then
      dr_bad "$_dr_proto: could not reach the server"
    else
      dr_bad "$_dr_proto: no model on this account could answer"
    fi
  fi
done

if [ -f "$DR_NET_FLAG" ]; then
  dr_say ""
  dr_bad "Cannot reach DeepRouter at $DR_BASE_URL."
  dr_note "Check your network connection, then copy a fresh command and try again."
  dr_say ""
  exit 1
fi

if [ -f "$DR_AUTH_FLAG" ]; then
  dr_say ""
  dr_bad "That key was rejected."
  dr_note "Generate a new key on your API keys page and copy a fresh command."
  dr_say ""
  exit 1
fi

# Codex and Gemini never get deeprouter-auto: their request bodies use `input`
# and `contents`, the router reads neither, and it silently falls back to a
# default model instead of failing (PRD 4.3). Named models are the honest
# choice until that is fixed.
dr_model_for() {
  eval "_dr_m=\${DR_MODEL_$(dr_tool_proto "$1")}"
  if [ "$_dr_m" = "deeprouter-auto" ]; then
    case "$1" in
      codex|gemini-cli)
        _dr_alt=$(dr_candidates "$(dr_tool_proto "$1")" 2>/dev/null | head -n 1)
        [ -n "$_dr_alt" ] && printf '%s' "$_dr_alt" || printf '%s' "$_dr_m"
        return ;;
    esac
  fi
  printf '%s' "$_dr_m"
}

DR_CONFIGURABLE=""
for _dr_t in $DR_TARGETS; do
  [ -n "$(dr_model_for "$_dr_t")" ] && DR_CONFIGURABLE="$DR_CONFIGURABLE $_dr_t"
done

if [ -z "$(printf '%s' "$DR_CONFIGURABLE" | tr -d ' ')" ]; then
  dr_say ""
  dr_bad "No usable model was found for the tools you selected."
  if [ -s "$DR_FUNDS_FILE" ]; then
    dr_note "Your balance is \$$(cut -f1 "$DR_FUNDS_FILE") and the cheapest model needs \$$(cut -f2 "$DR_FUNDS_FILE") held aside."
    dr_note "Top up here: $DR_BASE_URL/topup"
  else
    dr_note "Check that your key is allowed to use at least one chat model."
  fi
  dr_say ""
  exit 1
fi

# ===========================================================================
# Step 3 - back up, then merge
# ===========================================================================

DR_MANIFEST_TOOLS=""
DR_ENV_VARS=""
DR_WROTE=""
DR_FAILED=""
DR_CODEX_PROFILE=0

mkdir -p "$DR_DIR" 2>/dev/null || {
  dr_bad "Cannot create $DR_DIR"
  exit 1
}
chmod 700 "$DR_DIR" 2>/dev/null

# Carry the first install forward. A second run must not overwrite
# original_backup - the earliest copy is the only one that is really the
# user's own state, and uninstall restores from it (PRD 4.6).
dr_prior() {
  [ -f "$DR_MANIFEST" ] || return 1
  dr_json_valid "$DR_MANIFEST" || return 1
  dr_json_flat "$DR_MANIFEST" | awk -v SEP="$DR_SEP" -v want="$1" -v field="$2" -F'\t' '
    { nf = split($1, a, SEP)
      if (a[1] != "tools" || nf < 3) next
      v = $2; gsub(/^"|"$/, "", v)
      if (a[3] == "name") name[a[2]] = v
      else if (a[3] == field) val[a[2]] = v }
    END { for (i in name) if (name[i] == want) { print val[i]; exit } }'
}

dr_backup() {
  _dr_file=$1
  [ -f "$_dr_file" ] || return 1
  # $$ as well as the stamp: DR_STAMP has one-second resolution, so two runs
  # in the same second named their backups identically and the second cp
  # destroyed the first run's copy of the user's true original - while the
  # manifest kept pointing at the now-clobbered path. Found by CI, where the
  # whole install-install-uninstall scenario fits inside one second.
  _dr_bak="$_dr_file.bak-$DR_STAMP-$$"
  cp "$_dr_file" "$_dr_bak" 2>/dev/null || return 1
  printf '%s' "$_dr_bak"
}

# dr_record TOOL FILE PRE_EXISTING BACKUP
dr_record() {
  _dr_prior_bak=$(dr_prior "$1" "original_backup" 2>/dev/null)
  _dr_prior_pre=$(dr_prior "$1" "pre_existing" 2>/dev/null)
  [ "$_dr_prior_bak" = "null" ] && _dr_prior_bak=""
  if [ -n "$_dr_prior_pre" ]; then
    # Seen before: keep the first run's truth about this file.
    _dr_pre=$_dr_prior_pre
    _dr_bak=$_dr_prior_bak
  else
    _dr_pre=$3
    _dr_bak=$4
  fi
  # A dash stands in for every empty value. These rows are read back with
  # `read`, which splits on IFS, and a tab is an IFS whitespace character -
  # so two adjacent tabs collapse into one and every later field shifts left.
  # Claude Code has no file by design, and that gap silently wrote its
  # pre_existing flag into the manifest where its filename belongs.
  DR_MANIFEST_TOOLS="$DR_MANIFEST_TOOLS$(printf '%s\t%s\t%s\t%s\n' "$1" "${2:--}" "$_dr_pre" "${_dr_bak:--}")
"
}

dr_add_env() { DR_ENV_VARS="$DR_ENV_VARS $1=$2"; }

# --- Claude Code: environment only ----------------------------------------
# PRD 0.1 F1: the env block inside .claude/settings.json does not work - it
# still asks you to log in. Only real environment variables do. A pleasant
# side effect is that we never touch that file, so the user's permissions,
# hooks and theme are out of reach by construction.
dr_setup_claude_code() {
  _dr_model=$(dr_model_for claude-code)
  dr_add_env ANTHROPIC_BASE_URL "$DR_BASE_URL"
  dr_add_env ANTHROPIC_AUTH_TOKEN "$DR_API_KEY"
  dr_add_env ANTHROPIC_MODEL "$_dr_model"
  dr_add_env ANTHROPIC_SMALL_FAST_MODEL "$_dr_model"
  dr_record claude-code "" true ""
  dr_ok "Claude Code    environment variables"
  return 0
}

# --- OpenCode: merge into its JSON ----------------------------------------
dr_opencode_config() {
  _dr_p=$(opencode debug paths 2>/dev/null |
    sed -n 's/.*[Cc]onfig[^/]*\(\/[^ ]*\.json\).*/\1/p' | head -n 1)
  if [ -n "$_dr_p" ]; then printf '%s' "$_dr_p"; return; fi
  printf '%s' "${XDG_CONFIG_HOME:-$HOME/.config}/opencode/opencode.json"
}

dr_setup_opencode() {
  _dr_file=$(dr_opencode_config)
  _dr_model=$(dr_model_for opencode)
  _dr_pre=false
  _dr_bak=""

  mkdir -p "$(dirname "$_dr_file")" 2>/dev/null
  if [ -f "$_dr_file" ]; then
    _dr_pre=true
    if ! dr_json_valid "$_dr_file"; then
      dr_bad "OpenCode       $_dr_file is not valid JSON - left untouched"
      dr_note "Fix or move that file, then copy a fresh command from your keys page."
      return 1
    fi
    _dr_bak=$(dr_backup "$_dr_file") || {
      dr_bad "OpenCode       cannot back up $_dr_file - left untouched"
      return 1
    }
  else
    printf '{}\n' > "$_dr_file" 2>/dev/null || {
      dr_bad "OpenCode       cannot write $_dr_file"
      return 1
    }
  fi

  {
    printf 'provider%sdeeprouter\t' "$DR_SEP"
    printf '{"npm":"@ai-sdk/openai-compatible","name":"DeepRouter",'
    printf '"options":{"baseURL":%s,"apiKey":%s},' \
      "$(dr_str "$DR_BASE_URL/v1")" "$(dr_str "$DR_API_KEY")"
    printf '"models":{%s:{"name":"DeepRouter"}}}\n' "$(dr_str "$_dr_model")"
    # Top-level default too. With only a provider entry, OpenCode still starts
    # on whatever model its own catalog picks (seen live 2026-08-28: Nano
    # Banana Pro, demanding a Google key). Overwrites a user-set default on
    # purpose - pointing the tool at DeepRouter is what running this means.
    printf 'model\t%s\n' "$(dr_str "deeprouter/$_dr_model")"
  } > "$DR_TMP/set.opencode"

  if ! dr_json_edit "$_dr_file" "$DR_TMP/set.opencode" > "$DR_TMP/out.opencode"; then
    dr_bad "OpenCode       could not rewrite $_dr_file - left untouched"
    return 1
  fi
  [ -s "$DR_TMP/out.opencode" ] || {
    dr_bad "OpenCode       produced an empty config - left untouched"
    return 1
  }
  # Subshell: a failed redirect is reported by the shell itself, and only
  # a redirect around the whole subshell can silence that line.
  if ! ( cat "$DR_TMP/out.opencode" > "$_dr_file" ) 2>/dev/null; then
    dr_bad "OpenCode       cannot write $_dr_file"
    dr_note "The file is not writable - fix its permissions, then copy a fresh command."
    return 1
  fi

  dr_record opencode "$_dr_file" "$_dr_pre" "$_dr_bak"
  if [ -n "$_dr_bak" ]; then
    dr_ok "OpenCode       $_dr_file (backed up)"
  else
    dr_ok "OpenCode       $_dr_file"
  fi
  return 0
}

# --- Codex: a whole separate file when one already exists ------------------
# PRD Q7, measured: a user who already configured Codex has an opinion about
# their default model, and hijacking it is rude. Writing a separate profile
# file also removes the need to edit TOML at all - which was the single largest
# engineering risk in this card.
dr_codex_body() {
  printf 'model = "%s"\n' "$1"
  printf 'model_provider = "deeprouter"\n\n'
  printf '[model_providers.deeprouter]\n'
  printf 'name = "DeepRouter"\n'
  printf 'base_url = "%s/v1"\n' "$DR_BASE_URL"
  printf 'env_key = "DEEPROUTER_API_KEY"\n'
  # `wire_api = "chat"` was removed in v0.149.1 and now stops Codex loading
  # its config at all (PRD 0.1 F13).
  printf 'wire_api = "responses"\n'
}

dr_setup_codex() {
  _dr_main="$HOME/.codex/config.toml"
  _dr_model=$(dr_model_for codex)
  mkdir -p "$HOME/.codex" 2>/dev/null

  dr_add_env DEEPROUTER_API_KEY "$DR_API_KEY"

  if [ -f "$_dr_main" ]; then
    _dr_file="$HOME/.codex/deeprouter.config.toml"
    DR_CODEX_PROFILE=1
    _dr_pre=false
    [ -f "$_dr_file" ] && _dr_pre=true
    _dr_bak=""
    [ "$_dr_pre" = "true" ] && _dr_bak=$(dr_backup "$_dr_file")
    # The same content as the standalone case, in a file Codex layers on top
    # when asked. `--profile <name>` means "layer $CODEX_HOME/<name>.config.toml
    # over the base config", so these keys belong at the TOP level - wrapping
    # them in a [profiles.deeprouter] section buries them in a nested profile
    # nothing activates, and Codex quietly keeps using the base provider.
    # Measured 2026-08-28 on codex-cli 0.149.1: that one header line was the
    # difference between `provider: deeprouter` and `provider: openai`.
    dr_codex_body "$_dr_model" > "$_dr_file" 2>/dev/null || {
      dr_bad "Codex CLI      cannot write $_dr_file"
      return 1
    }
    dr_record codex "$_dr_file" "$_dr_pre" "$_dr_bak"
    dr_ok "Codex CLI      $_dr_file (your config.toml was not touched)"
  else
    _dr_file=$_dr_main
    dr_codex_body "$_dr_model" > "$_dr_file" 2>/dev/null || {
      dr_bad "Codex CLI      cannot write $_dr_file"
      return 1
    }
    dr_record codex "$_dr_file" false ""
    dr_ok "Codex CLI      $_dr_file"
  fi
  return 0
}

# --- Gemini CLI: JSON and environment, both required -----------------------
# PRD 0.1 F10: without security.auth.selectedType the CLI refuses before
# sending anything - and it is a nested key, the old flat one does nothing.
# F12: the model name lives here too; GEMINI_MODEL is never read.
dr_setup_gemini() {
  _dr_file="$HOME/.gemini/settings.json"
  _dr_model=$(dr_model_for gemini-cli)
  _dr_pre=false
  _dr_bak=""

  mkdir -p "$HOME/.gemini" 2>/dev/null
  if [ -f "$_dr_file" ]; then
    _dr_pre=true
    if ! dr_json_valid "$_dr_file"; then
      dr_bad "Gemini CLI     $_dr_file is not valid JSON - left untouched"
      dr_note "Fix or move that file, then copy a fresh command from your keys page."
      return 1
    fi
    _dr_bak=$(dr_backup "$_dr_file") || {
      dr_bad "Gemini CLI     cannot back up $_dr_file - left untouched"
      return 1
    }
  else
    printf '{}\n' > "$_dr_file" 2>/dev/null || {
      dr_bad "Gemini CLI     cannot write $_dr_file"
      return 1
    }
  fi

  {
    printf 'security%sauth%sselectedType\t"gemini-api-key"\n' "$DR_SEP" "$DR_SEP"
    printf 'model%sname\t%s\n' "$DR_SEP" "$(dr_str "$_dr_model")"
  } > "$DR_TMP/set.gemini"

  if ! dr_json_edit "$_dr_file" "$DR_TMP/set.gemini" > "$DR_TMP/out.gemini"; then
    dr_bad "Gemini CLI     could not rewrite $_dr_file - left untouched"
    return 1
  fi
  [ -s "$DR_TMP/out.gemini" ] || {
    dr_bad "Gemini CLI     produced an empty config - left untouched"
    return 1
  }
  # Subshell: a failed redirect is reported by the shell itself, and only
  # a redirect around the whole subshell can silence that line.
  if ! ( cat "$DR_TMP/out.gemini" > "$_dr_file" ) 2>/dev/null; then
    dr_bad "Gemini CLI     cannot write $_dr_file"
    dr_note "The file is not writable - fix its permissions, then copy a fresh command."
    return 1
  fi

  dr_add_env GEMINI_API_KEY "$DR_API_KEY"
  # Never with /v1beta - the CLI appends it, and the doubled path is rejected
  # by the gateway with a message that says nothing about the cause (F11).
  dr_add_env GOOGLE_GEMINI_BASE_URL "$DR_BASE_URL"
  dr_record gemini-cli "$_dr_file" "$_dr_pre" "$_dr_bak"
  if [ -n "$_dr_bak" ]; then
    dr_ok "Gemini CLI     $_dr_file (backed up)"
  else
    dr_ok "Gemini CLI     $_dr_file"
  fi
  return 0
}

dr_say ""
dr_say "  Writing configuration..."

for _dr_t in $DR_CONFIGURABLE; do
  case "$_dr_t" in
    claude-code) dr_setup_claude_code ;;
    opencode)    dr_setup_opencode ;;
    codex)       dr_setup_codex ;;
    gemini-cli)  dr_setup_gemini ;;
  esac
  if [ $? -eq 0 ]; then DR_WROTE="$DR_WROTE $_dr_t"; else DR_FAILED="$DR_FAILED $_dr_t"; fi
done

# ===========================================================================
# Step 4 - our own env file, and exactly one line in the user's shell config
# ===========================================================================
# rustup's pattern (PRD 3 D2). Appending exports directly would pile up on
# every re-run, would need editing whenever the key changes, and would leave
# uninstall hunting through a file it does not own. One line referencing a
# file we rewrite wholesale is idempotent for free.

dr_shell_kind() {
  _dr_s=$(basename "${SHELL:-/bin/sh}" 2>/dev/null)
  case "$_dr_s" in
    bash|zsh|fish|ksh|dash|sh) printf '%s' "$_dr_s" ;;
    *) printf 'unknown' ;;
  esac
}

dr_shell_rc() {
  case "$1" in
    zsh)  printf '%s' "${ZDOTDIR:-$HOME}/.zshrc" ;;
    fish) printf '%s' "${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
    bash)
      # A login shell on macOS reads .bash_profile and never .bashrc, so
      # writing only the latter would look like nothing happened.
      if [ -f "$HOME/.bash_profile" ]; then printf '%s' "$HOME/.bash_profile"
      else printf '%s' "$HOME/.bashrc"; fi ;;
    ksh|dash|sh) printf '%s' "$HOME/.profile" ;;
    *) printf '' ;;
  esac
}

DR_SHELL=$(dr_shell_kind)
DR_RC=$(dr_shell_rc "$DR_SHELL")
DR_RC_PRE=false
DR_ENV_FILE="$DR_DIR/env.sh"
[ "$DR_SHELL" = "fish" ] && DR_ENV_FILE="$DR_DIR/env.fish"
DR_RC_LINE=". \"$DR_ENV_FILE\""
[ "$DR_SHELL" = "fish" ] && DR_RC_LINE="source \"$DR_ENV_FILE\""

if [ -n "$DR_ENV_VARS" ]; then
  # Rewritten whole every run, never appended to - that is what makes repeat
  # runs produce an identical file.
  : > "$DR_ENV_FILE"
  chmod 600 "$DR_ENV_FILE" 2>/dev/null
  printf '# Written by DeepRouter one-click setup. Rewritten on every run.\n' >> "$DR_ENV_FILE"
  printf '# Undo: curl -fsSL %s/uninstall | sh\n' "$DR_BASE_URL" >> "$DR_ENV_FILE"
  for _dr_kv in $DR_ENV_VARS; do
    _dr_k=${_dr_kv%%=*}
    _dr_v=${_dr_kv#*=}
    if [ "$DR_SHELL" = "fish" ]; then
      printf 'set -x %s "%s"\n' "$_dr_k" "$_dr_v" >> "$DR_ENV_FILE"
    else
      printf 'export %s="%s"\n' "$_dr_k" "$_dr_v" >> "$DR_ENV_FILE"
    fi
  done
  chmod 600 "$DR_ENV_FILE" 2>/dev/null
  if [ -s "$DR_ENV_FILE" ]; then
    dr_ok "Environment    $DR_ENV_FILE"
  else
    dr_bad "Environment    cannot write $DR_ENV_FILE"
    dr_note "Fix that path's permissions - without this file the tools above have no key."
  fi

  if [ -z "$DR_RC" ]; then
    dr_bad "Shell          could not tell which shell you use"
    dr_note "Add this line to your shell startup file yourself:"
    dr_note "  $DR_RC_LINE"
  else
    mkdir -p "$(dirname "$DR_RC")" 2>/dev/null
    # Whether this file was already here decides what undoing means: rewriting
    # it without our line, or deleting a file that only ever held our line.
    # Without recording it, uninstall meets an empty file it cannot explain and
    # leaves it behind - measured on Linux, where SHELL=/bin/sh selects a
    # ~/.profile that most machines do not already have.
    [ -f "$DR_RC" ] && DR_RC_PRE=true
    if [ -f "$DR_RC" ] && grep -Fq "$DR_ENV_FILE" "$DR_RC" 2>/dev/null; then
      dr_ok "Shell          $DR_RC already references it"
    else
      # A file whose last line has no newline would otherwise get our line
      # glued onto the end of it.
      if [ -s "$DR_RC" ] && [ -n "$(tail -c 1 "$DR_RC" 2>/dev/null)" ]; then
        printf '\n' >> "$DR_RC"
      fi
      {
        printf '\n# Added by DeepRouter one-click setup\n'
        printf '%s\n' "$DR_RC_LINE"
      } >> "$DR_RC" 2>/dev/null && dr_ok "Shell          one line added to $DR_RC" ||
        dr_bad "Shell          cannot write $DR_RC"
    fi
  fi
fi

# ===========================================================================
# Step 5 - the manifest
# ===========================================================================
# Uninstall is only as good as this file. Without it, restoring is guesswork:
# a second run leaves two .bak- files and only the earliest is the user's real
# original, while a file we created ourselves has no backup at all and must be
# deleted rather than restored (PRD 4.6).

{
  printf '{\n'
  printf '  "installed_at": "%s",\n' "$DR_NOW"
  printf '  "base_url": %s,\n' "$(dr_str "$DR_BASE_URL")"
  printf '  "tools": [\n'
  _dr_first=1
  printf '%s' "$DR_MANIFEST_TOOLS" | while IFS="$(printf '\t')" read -r _dr_n _dr_f _dr_p _dr_b; do
    [ -n "$_dr_n" ] || continue
    [ "$_dr_first" = "1" ] || printf ',\n'
    _dr_first=0
    [ "$_dr_f" = "-" ] && _dr_f=""
    [ "$_dr_b" = "-" ] && _dr_b=""
    printf '    { "name": %s, "file": %s, "pre_existing": %s, "original_backup": %s }' \
      "$(dr_str "$_dr_n")" "$(dr_str "$_dr_f")" "${_dr_p:-false}" \
      "$([ -n "$_dr_b" ] && dr_str "$_dr_b" || printf 'null')"
  done
  printf '\n  ],\n'
  printf '  "shell": { "file": %s, "line": %s, "pre_existing": %s },\n' \
    "$(dr_str "$DR_RC")" "$(dr_str "$DR_RC_LINE")" "$DR_RC_PRE"
  printf '  "env_file": %s,\n' "$(dr_str "$DR_ENV_FILE")"
  printf '  "env_vars": ['
  _dr_first=1
  for _dr_kv in $DR_ENV_VARS; do
    [ "$_dr_first" = "1" ] || printf ', '
    _dr_first=0
    printf '%s' "$(dr_str "${_dr_kv%%=*}")"
  done
  printf ']\n}\n'
} > "$DR_MANIFEST" 2>/dev/null
chmod 600 "$DR_MANIFEST" 2>/dev/null
if [ ! -s "$DR_MANIFEST" ]; then
  dr_bad "Manifest       cannot write $DR_MANIFEST"
  dr_note "Setup itself worked, but uninstall will have no record of what to undo."
fi

# ===========================================================================
# Step 6 - prove it works, one protocol at a time
# ===========================================================================
# Four different base URLs and four different protocols, none of which
# complains at write time when it is wrong. One blanket request would prove
# only that the key is valid (PRD 4.4).

dr_verify_one() {
  case "$1" in
    anthropic)
      dr_http POST "$DR_BASE_URL/v1/messages" \
        "{\"model\":$(dr_str "$2"),\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" ;;
    openai)
      dr_http POST "$DR_BASE_URL/v1/chat/completions" \
        "{\"model\":$(dr_str "$2"),\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" ;;
    responses)
      dr_http POST "$DR_BASE_URL/v1/responses" \
        "{\"model\":$(dr_str "$2"),\"max_output_tokens\":16,\"input\":\"hi\"}" ;;
    gemini)
      dr_http POST "$DR_BASE_URL/v1beta/models/$2:generateContent" \
        "{\"contents\":[{\"role\":\"user\",\"parts\":[{\"text\":\"hi\"}]}]}" ;;
  esac
}

dr_verify_label() {
  case "$1" in
    anthropic) printf 'Anthropic        ' ;;
    openai)    printf 'OpenAI           ' ;;
    responses) printf 'OpenAI Responses ' ;;
    gemini)    printf 'Google Gemini    ' ;;
  esac
}

dr_tool_verify_proto() {
  case "$1" in
    claude-code) printf 'anthropic' ;;
    opencode)    printf 'openai' ;;
    codex)       printf 'responses' ;;
    gemini-cli)  printf 'gemini' ;;
  esac
}

DR_VERIFY_OK=0
DR_VERIFY_BAD=0

if [ -n "$(printf '%s' "$DR_WROTE" | tr -d ' ')" ]; then
  dr_say ""
  dr_say "  Verifying..."
  for _dr_t in $DR_WROTE; do
    _dr_vp=$(dr_tool_verify_proto "$_dr_t")
    _dr_lbl=$(dr_verify_label "$_dr_vp")
    _dr_name=$(dr_tool_name "$_dr_t")
    _dr_code=$(dr_verify_one "$_dr_vp" "$(dr_model_for "$_dr_t")")
    case "$(dr_classify "$_dr_code")" in
      ok)
        DR_VERIFY_OK=$((DR_VERIFY_OK + 1))
        dr_ok "$_dr_lbl works   ($_dr_name)" ;;
      busy)
        DR_VERIFY_OK=$((DR_VERIFY_OK + 1))
        dr_skip "$_dr_lbl the model is busy right now ($_dr_name)"
        dr_note "Your configuration is written and correct - just try again in a moment." ;;
      funds)
        DR_VERIFY_BAD=$((DR_VERIFY_BAD + 1))
        _dr_f=$(dr_funds_figures)
        dr_bad "$_dr_lbl not enough balance ($_dr_name)"
        if [ -n "$_dr_f" ]; then
          dr_note "You have \$$(printf '%s' "$_dr_f" | cut -f1); this model needs \$$(printf '%s' "$_dr_f" | cut -f2) held aside per request."
        fi
        dr_note "Top up here: $DR_BASE_URL/topup" ;;
      auth)
        DR_VERIFY_BAD=$((DR_VERIFY_BAD + 1))
        dr_bad "$_dr_lbl the key was rejected ($_dr_name)"
        dr_note "Generate a new key on your API keys page." ;;
      network)
        DR_VERIFY_BAD=$((DR_VERIFY_BAD + 1))
        dr_bad "$_dr_lbl could not reach DeepRouter ($_dr_name)"
        dr_note "Check your network and run the command again." ;;
      *)
        DR_VERIFY_BAD=$((DR_VERIFY_BAD + 1))
        dr_bad "$_dr_lbl did not work ($_dr_name)"
        _dr_m=$(dr_err_message)
        [ -n "$_dr_m" ] && dr_note "The gateway said: $_dr_m" ;;
    esac
  done
fi

# ===========================================================================
# Step 7 - the report
# ===========================================================================

dr_say ""
dr_say "  --------------------------------------------------------------"

DR_COUNT=$(printf '%s' "$DR_WROTE" | wc -w | tr -d ' ')
if [ "$DR_COUNT" = "0" ]; then
  dr_say "  Nothing was configured."
else
  dr_say "  Done. $DR_COUNT tool(s) configured."
fi

if [ -n "$(printf '%s' "$DR_FAILED" | tr -d ' ')" ]; then
  dr_say ""
  for _dr_t in $DR_FAILED; do
    dr_note "$(dr_tool_name "$_dr_t") was left as it was - see the message above."
  done
fi

# Only OpenCode reads its configuration at startup. The other three read
# environment variables, and a shell only picks those up in a genuinely new
# window (PRD 2.2).
DR_NEEDS_RESTART=""
for _dr_t in $DR_WROTE; do
  [ "$_dr_t" = "opencode" ] || DR_NEEDS_RESTART="$DR_NEEDS_RESTART $_dr_t"
done

# One table, one row per configured tool: the exact thing to type, and when.
# This exists because a real user typed plain `codex` after installing, met the
# ChatGPT login screen, and concluded setup had failed - the old prose hint was
# printed and still not seen (2026-08-28).
if [ "$DR_COUNT" != "0" ]; then
  dr_say ""
  dr_say "  How to start each tool:"
  dr_say ""
  for _dr_t in $DR_WROTE; do
    case "$_dr_t" in
      opencode)
        dr_say "    OpenCode      type: opencode                     works right now" ;;
      claude-code)
        dr_say "    Claude Code   type: claude                       new terminal first" ;;
      gemini-cli)
        dr_say "    Gemini CLI    type: gemini                       new terminal first" ;;
      codex)
        if [ "$DR_CODEX_PROFILE" = "1" ]; then
          dr_say "    Codex CLI     type: codex --profile deeprouter   new terminal first"
        else
          dr_say "    Codex CLI     type: codex                        new terminal first"
        fi ;;
    esac
  done
  if [ -n "$(printf '%s' "$DR_NEEDS_RESTART" | tr -d ' ')" ]; then
    dr_say ""
    dr_say "  ! \"New terminal first\" means: Close this window completely and"
    dr_say "    open a new one - starting another terminal from inside this"
    dr_say "    one is not enough."
    [ -n "$DR_RC" ] && dr_say "    Or make this window work right now:  . \"$DR_ENV_FILE\""
  fi
fi

# The one start command that fails in a misleading way gets shouted. Plain
# codex does not error - it shows the ChatGPT login screen, which reads as
# "setup did not work" to exactly the person this product is for.
if [ "$DR_CODEX_PROFILE" = "1" ]; then
  dr_say ""
  dr_say "  !!! CODEX - READ THIS !!!"
  dr_say "  !!! Typing plain codex will STILL show the ChatGPT login screen."
  dr_say "  !!! You already had Codex settings, so they were left untouched"
  dr_say "  !!! and DeepRouter went into a separate profile."
  dr_say "  !!! To use DeepRouter, ALWAYS start it as:"
  dr_say "  !!!"
  dr_say "  !!!     codex --profile deeprouter"
fi

if dr_in_list gemini-cli "$DR_WROTE"; then
  dr_say ""
  dr_say "  !!! GEMINI - READ THIS !!!"
  dr_say "  !!! The first time gemini starts it may ask how you want to"
  dr_say "  !!! sign in. Pick the \"API key\" option - \"Login with Google\""
  dr_say "  !!! would switch it away from DeepRouter."
fi

if dr_in_list claude-code "$DR_WROTE"; then
  dr_say ""
  dr_say "  ! The first time you run claude it asks \"Is this a project you"
  dr_say "    trust?\" - press Enter. You do not need an Anthropic account."
  # The one thing this setup takes away. Not saying it means the user finds
  # out later and has no way to connect it to us (PRD 0.1 F15).
  dr_say "  ! Claude Code will show \"claude.ai connectors are disabled\" at the"
  dr_say "    bottom. That is expected while it runs through DeepRouter, and"
  dr_say "    uninstalling brings them back."
fi

dr_say ""
dr_say "  Undo anytime:  curl -fsSL $DR_BASE_URL/uninstall | sh"
dr_say ""

[ "$DR_VERIFY_BAD" -gt 0 ] && [ "$DR_VERIFY_OK" -eq 0 ] && exit 1
exit 0
