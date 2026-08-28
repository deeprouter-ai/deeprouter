#!/bin/sh
# DeepRouter uninstall (macOS / Linux / WSL / Git Bash).
#
# Puts this machine back the way it was before one-click setup ran. It needs no
# token and no key, which is why it is a fixed address that always works:
#
#   curl -fsSL <base>/uninstall | sh
#
# Everything it does comes from ~/.deeprouter/installed.json. Without that file
# it does nothing at all - guessing which lines in your shell config or which
# config files were ours is exactly the kind of damage this script exists to
# avoid (PRD 4.6).

set -u

DR_DIR="$HOME/.deeprouter"
DR_MANIFEST="$DR_DIR/installed.json"
DR_SEP=$(printf '\037')

dr_say()  { printf '%s\n' "$*"; }
dr_ok()   { printf '  [ ok ] %s\n' "$*"; }
dr_none() { printf '  [ -- ] %s\n' "$*"; }
dr_bad()  { printf '  [fail] %s\n' "$*"; }
dr_note() { printf '         %s\n' "$*"; }

# Same reader as setup.sh, flatten mode only: this script never writes JSON.
DR_AWK_JSON='
function fail(m) { print "ERR " m > "/dev/stderr"; exit 2 }
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
    if (c == "n") o = o "\n"; else if (c == "t") o = o "\t"; else o = o c
  }
  return o
}
function pliteral(   st, c, id) {
  st = p
  while (p <= n) { c = substr(s, p, 1); if (index(" \t\r\n,}]", c) > 0) break; p++ }
  if (p == st) fail("empty value")
  id = newnode("lit"); raw[id] = substr(s, st, p - st)
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
function flat(id, path,   i, np) {
  if (kind[id] == "obj") {
    for (i = 1; i <= nk[id]; i++) flat(kid[id, i], (path == "" ? key[id, i] : path SEP key[id, i]))
    return
  }
  if (kind[id] == "arr") {
    for (i = 1; i <= nk[id]; i++) flat(kid[id, i], (path == "" ? (i - 1) : path SEP (i - 1)))
    return
  }
  print path "\t" raw[id]
}
BEGIN { SEP = sprintf("%c", 31); nid = 0 }
{ doc = doc $0 "\n" }
END { s = doc; p = 1; n = length(s); root = pvalue(); flat(root, "") }
'

dr_flat() { awk "$DR_AWK_JSON" "$1" 2>/dev/null; }
dr_unstr() { printf '%s' "$1" | sed -e 's/^"//' -e 's/"$//' -e 's/\\"/"/g' -e 's/\\\\/\\/g'; }

dr_field() {
  dr_flat "$DR_MANIFEST" | awk -F'\t' -v want="$1" '$1 == want { print $2; exit }'
}

dr_say ""
dr_say "DeepRouter uninstall"
dr_say ""

if [ ! -f "$DR_MANIFEST" ]; then
  dr_say "  No DeepRouter setup was found on this machine."
  dr_say "  Nothing was changed."
  dr_say ""
  dr_say "  (Looked for $DR_MANIFEST. Without it there is no record of what"
  dr_say "   to undo, and guessing would risk your own configuration.)"
  dr_say ""
  exit 0
fi

if ! dr_flat "$DR_MANIFEST" >/dev/null 2>&1; then
  dr_bad "$DR_MANIFEST is unreadable."
  dr_note "Nothing was changed. Remove that file by hand if you want to start over."
  dr_say ""
  exit 1
fi

DR_TMP=$(mktemp -d 2>/dev/null || printf '%s' "${TMPDIR:-/tmp}/dr-uninstall-$$")
mkdir -p "$DR_TMP" 2>/dev/null
trap 'rm -rf "$DR_TMP"' EXIT INT TERM
dr_flat "$DR_MANIFEST" > "$DR_TMP/flat"

# --- 1. tool configuration files -------------------------------------------
# pre_existing decides between two opposite actions, and getting it backwards
# is destructive either way: restoring a file we invented leaves a stale one
# behind, deleting a file the user already had loses their settings.

dr_say "  Restoring configuration..."

awk -v SEP="$DR_SEP" -F'\t' '
  { nf = split($1, a, SEP)
    if (a[1] != "tools" || nf < 3) next
    v = $2; gsub(/^"|"$/, "", v)
    rec[a[2]] = rec[a[2]]
    if (a[3] == "name") name[a[2]] = v
    else if (a[3] == "file") file[a[2]] = v
    else if (a[3] == "pre_existing") pre[a[2]] = v
    else if (a[3] == "original_backup") bak[a[2]] = v
    idx[a[2]] = 1 }
  END {
    # A placeholder for every empty value. `read` splits on IFS, and a tab is an
    # IFS whitespace character, so two adjacent tabs collapse into one and every
    # field after the gap shifts left - which showed up on Linux as Claude Code
    # (whose file is deliberately empty) reporting its pre_existing flag where
    # its filename should be.
    for (i in idx) {
      f = (file[i] == "" ? "-" : file[i])
      b = (bak[i] == "" ? "-" : bak[i])
      print name[i] "\t" f "\t" pre[i] "\t" b
    }
  }
' "$DR_TMP/flat" > "$DR_TMP/tools"

while IFS="$(printf '\t')" read -r _dr_name _dr_file _dr_pre _dr_bak; do
  [ -n "$_dr_name" ] || continue
  # Claude Code has no file - it was only ever environment variables.
  [ "$_dr_file" = "-" ] && continue
  [ -n "$_dr_file" ] || continue
  if [ "$_dr_pre" = "true" ]; then
    if [ -n "$_dr_bak" ] && [ "$_dr_bak" != "null" ] && [ "$_dr_bak" != "-" ] && [ -f "$_dr_bak" ]; then
      if cp "$_dr_bak" "$_dr_file" 2>/dev/null; then
        dr_ok "$_dr_name  restored from the copy taken before the first install"
        rm -f "$_dr_bak" 2>/dev/null
      else
        dr_bad "$_dr_name  could not restore $_dr_file"
        dr_note "Your original is still at $_dr_bak"
      fi
    else
      dr_bad "$_dr_name  the original copy is missing - $_dr_file left as it is"
      dr_note "Edit it by hand to remove the DeepRouter entries."
    fi
  else
    # We created this file, so there is nothing to restore - removing it is
    # the only way back to "as if we had never run".
    if [ -f "$_dr_file" ]; then
      rm -f "$_dr_file" 2>/dev/null && dr_ok "$_dr_name  removed $_dr_file (we created it)" ||
        dr_bad "$_dr_name  could not remove $_dr_file"
    else
      dr_none "$_dr_name  $_dr_file is already gone"
    fi
  fi
done < "$DR_TMP/tools"

# --- 2. the one line in the shell config ------------------------------------
# Only ours comes out. Everything else in that file has to survive byte for
# byte - it is the user's, and we only ever added one line to it.

DR_RC=$(dr_unstr "$(dr_field shell${DR_SEP}file)")
DR_RC_PRE=$(dr_field "shell${DR_SEP}pre_existing")
DR_ENV_FILE=$(dr_unstr "$(dr_field env_file)")

if [ -n "$DR_RC" ] && [ -f "$DR_RC" ] && [ -n "$DR_ENV_FILE" ]; then
  if grep -Fq "$DR_ENV_FILE" "$DR_RC" 2>/dev/null; then
    awk -v marker="$DR_ENV_FILE" '
      # Setup appends exactly three lines - a blank, a comment, the reference -
      # so all three come out, in that order, and nothing else is even looked
      # at. Leaving the blank behind would mean the file is not byte-identical
      # to the one we found, which is the whole promise of uninstall.
      { lines[NR] = $0 }
      END {
        for (i = 1; i <= NR; i++) {
          if (index(lines[i], marker) == 0) continue
          drop[i] = 1
          j = i - 1
          if (j >= 1 && lines[j] ~ /^# Added by DeepRouter/) { drop[j] = 1; j-- }
          if (j >= 1 && lines[j] == "") drop[j] = 1
        }
        for (i = 1; i <= NR; i++) if (!(i in drop)) print lines[i]
      }
    ' "$DR_RC" > "$DR_TMP/rc" 2>/dev/null
    if [ ! -s "$DR_TMP/rc" ] && [ "$DR_RC_PRE" != "true" ]; then
      # Nothing of theirs was ever in here - the file exists only because we
      # made it. Emptying it would leave a stray file where none had been.
      rm -f "$DR_RC" 2>/dev/null && dr_ok "Shell         removed $DR_RC (we created it)" ||
        dr_bad "Shell         could not remove $DR_RC"
    elif [ -s "$DR_TMP/rc" ] || [ ! -s "$DR_RC" ]; then
      cat "$DR_TMP/rc" > "$DR_RC" && dr_ok "Shell         our line removed from $DR_RC" ||
        dr_bad "Shell         could not rewrite $DR_RC"
    else
      # Their file, and taking our line out would empty it. That does not add
      # up, so change nothing and say so rather than guess.
      dr_bad "Shell         refused to rewrite $DR_RC - it would have been emptied"
    fi
  else
    dr_none "Shell         $DR_RC no longer references us"
  fi
fi

# --- 3. our own directory ----------------------------------------------------

if [ -d "$DR_DIR" ]; then
  rm -rf "$DR_DIR" 2>/dev/null && dr_ok "Removed $DR_DIR" || dr_bad "Could not remove $DR_DIR"
fi

dr_say ""
dr_say "  --------------------------------------------------------------"
dr_say "  Done. This machine is back to how it was before setup ran."
dr_say ""
dr_say "  ! Open a new terminal - this one still has the old variables in"
dr_say "    its environment until you do."
dr_say ""
exit 0
