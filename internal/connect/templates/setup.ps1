# DeepRouter one-click setup (Windows PowerShell 5.1+).
#
# This is the template. The server injects your base URL, your API key and the
# tools you ticked into the marked block below, then serves the result.
#
# It is the twin of setup.sh and must behave identically: same detection, same
# backups, same merge rules, same wording. Where the two differ it is because
# Windows differs, and each of those places says so.
#
# Undo everything:  irm <base>/uninstall | iex

# ASCII only, deliberately. PowerShell 5.1 reads a saved .ps1 in the system
# ANSI codepage, so one non-ASCII byte can swallow a quote and break parsing
# outright - and the two-step form we recommend for security saves this to a
# file before running it (PRD 0.1 F6).

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# ---------------------------------------------------------------------------
# @@DEEPROUTER_INJECT@@
# ---------------------------------------------------------------------------

$DrDir      = Join-Path $env:USERPROFILE '.deeprouter'
$DrManifest = Join-Path $DrDir 'installed.json'
$DrStamp    = Get-Date -Format 'yyyyMMdd-HHmmss'
$DrNow      = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

# ===========================================================================
# Output
# ===========================================================================

function Dr-Say  { param([string]$m = '') Write-Host $m }
function Dr-Ok   { param([string]$m) Write-Host "  [ ok ] $m" }
function Dr-Skip { param([string]$m) Write-Host "  [skip] $m" }
function Dr-None { param([string]$m) Write-Host "  [ -- ] $m" }
function Dr-Bad  { param([string]$m) Write-Host "  [fail] $m" }
function Dr-Note { param([string]$m) Write-Host "         $m" }

# ===========================================================================
# Tool table - the same three facts as setup.sh, in the same order
# ===========================================================================

$DrTools = [ordered]@{
  'claude-code' = @{
    Name  = 'Claude Code'
    Exe   = 'claude'
    Login = (Join-Path $env:USERPROFILE '.claude\.credentials.json')
    Proto = 'anthropic'
    # PRD 0.1 F20: the gateway holds price x max_tokens aside before it runs
    # anything, so a tiny probe proves only that the key works. These are the
    # low end of what each tool really sends.
    MaxTokens = 8192
    Verify = 'anthropic'
  }
  'opencode' = @{
    Name = 'OpenCode'; Exe = 'opencode'
    Login = (Join-Path $env:USERPROFILE '.local\share\opencode\auth.json')
    Proto = 'openai'; MaxTokens = 4096; Verify = 'openai'
  }
  'codex' = @{
    Name = 'Codex CLI'; Exe = 'codex'
    Login = (Join-Path $env:USERPROFILE '.codex\auth.json')
    Proto = 'openai'; MaxTokens = 4096; Verify = 'responses'
  }
  'gemini-cli' = @{
    Name = 'Gemini CLI'; Exe = 'gemini'
    Login = (Join-Path $env:USERPROFILE '.gemini\oauth_creds.json')
    # Not 'gemini': it reaches the gateway over the Gemini protocol, but no
    # model in any deployment declares gemini support and the gateway
    # converts (PRD 0.1 F19).
    Proto = 'openai'; MaxTokens = 4096; Verify = 'gemini'
  }
}

# ===========================================================================
# Arguments
# ===========================================================================
# `irm | iex` leaves no terminal to ask a question through, so the default has
# to be safe on its own and choices arrive as flags (PRD 4.2.2):
#   & ([scriptblock]::Create((irm <url>))) -Force claude-code

$DrOnly  = @()
$DrForce = @()
for ($i = 0; $i -lt $args.Count; $i++) {
  switch -Regex ($args[$i]) {
    '^--?only$'  { $i++; if ($i -lt $args.Count) { $DrOnly  = $args[$i] -split '[, ]+' } }
    '^--?force$' { $i++; if ($i -lt $args.Count) { $DrForce = $args[$i] -split '[, ]+' } }
    '^--?all$'   { $DrForce = @($DrTools.Keys) }
  }
}

# ===========================================================================
# HTTP
# ===========================================================================

function Invoke-DrHttp {
  param([string]$Method, [string]$Url, [string]$Body)

  $headers = @{
    'Authorization'     = "Bearer $DrApiKey"
    'x-api-key'         = $DrApiKey
    'anthropic-version' = '2023-06-01'
  }
  try {
    if ($Body) {
      # Explicit UTF-8 bytes. Handing a string to -Body lets PowerShell pick
      # an encoding, which turns non-ASCII into question marks without ever
      # reporting an error (PRD 0.1 F6).
      $bytes = [System.Text.Encoding]::UTF8.GetBytes($Body)
      $resp = Invoke-WebRequest -Method $Method -Uri $Url -Headers $headers `
        -ContentType 'application/json' -Body $bytes -UseBasicParsing -ErrorAction Stop
    } else {
      $resp = Invoke-WebRequest -Method $Method -Uri $Url -Headers $headers `
        -UseBasicParsing -ErrorAction Stop
    }
    return @{ Code = [int]$resp.StatusCode; Body = [string]$resp.Content }
  } catch {
    # Invoke-RestMethod throws away the response body and leaves only
    # "(401) Unauthorized" - and every failure message in PRD 6 is built from
    # what the gateway actually said, so the stream has to be read by hand
    # (PRD 0.1 F7).
    $err = $_
    $r = $null
    try { $r = $err.Exception.Response } catch { }
    if ($null -ne $r) {
      $code = 0
      try { $code = [int]$r.StatusCode } catch { }

      # ErrorDetails first, the stream only as a fallback. Reading the response
      # stream is the documented way round Invoke-RestMethod eating the body
      # (PRD 0.1 F7), but Invoke-WebRequest has already read it: the stream
      # reports CanRead and CanSeek, then hands back zero bytes because it is
      # sitting at the end. Measured 2026-08-28 - and the failure is invisible,
      # because an empty message classifies as an unknown 4xx, so every reserve
      # failure quietly becomes "the token may not use this model". That is
      # every message-based rule in PRD 6, gone on Windows only.
      $text = ''
      try { if ($err.ErrorDetails -and $err.ErrorDetails.Message) { $text = [string]$err.ErrorDetails.Message } } catch { }
      if (-not $text) {
        try {
          $stream = $r.GetResponseStream()
          if ($stream.CanSeek) { $stream.Position = 0 }
          $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8)
          $text = $reader.ReadToEnd()
          $reader.Close()
        } catch { }
      }
      return @{ Code = $code; Body = $text }
    }
    return @{ Code = 0; Body = '' }
  }
}

# The gateway reports these three cases in Chinese, and this file has to stay
# ASCII (see the header), so its words are built from code points. They are the
# same three strings setup.sh matches literally - keep the two in step.
#   no-permission  = the token may not use this model
#   reserve / quota = priced out for this account
$DrZhNoPermission = [string][char]0x65E0 + [string][char]0x6743
$DrZhReserve      = [string][char]0x9884 + [string][char]0x6263 + [string][char]0x8D39
$DrZhQuota        = [string][char]0x989D + [string][char]0x5EA6

function Get-DrErrMessage {
  param($Resp)
  if (-not $Resp.Body) { return '' }
  try {
    $o = $Resp.Body | ConvertFrom-Json
    if ($o.error -and $o.error.message) { return [string]$o.error.message }
    if ($o.message) { return [string]$o.message }
  } catch { }
  return ''
}

# The same five outcomes setup.sh distinguishes. PRD 4.3 needs them apart:
# all three of whitelist/missing/funds arrive as 4xx from one deployment, and
# only one of them is worth telling the user about.
function Get-DrClass {
  param($Resp)
  switch ($Resp.Code) {
    200 { return 'ok' }
    401 { return 'auth' }
    402 { return 'funds' }
    503 { return 'busy' }
    529 { return 'busy' }
    0   { return 'network' }
  }
  $m = Get-DrErrMessage $Resp
  if ($m -match "$DrZhNoPermission|not allowed|no permission")   { return 'whitelist' }
  if ($m -match "$DrZhReserve|$DrZhQuota|insufficient|quota")    { return 'funds' }
  if ($m -match "does not exist|not found|model_not_found")      { return 'missing' }
  if ($Resp.Code -eq 403) { return 'whitelist' }
  if ($Resp.Code -eq 404) { return 'missing' }
  return 'other'
}

# The gateway answers in Chinese and puts the two numbers a user needs inside
# the sentence. We pull the numbers out instead of echoing it: this script is
# ASCII, and PRD 6 asks for those figures, not for that wording.
function Get-DrFunds {
  param($Resp)
  $m = Get-DrErrMessage $Resp
  # The currency symbol depends on the gateway's display setting (fullwidth
  # dollar by default; yen or a custom symbol are possible) - so key on no
  # symbol at all: strip the digit-filled '(request id: ...)' tail, then
  # take the last two numbers. Found live: the ASCII-$ match never fired.
  $m = $m -replace '\(request id[^)]*\)', ''
  $hit = [regex]::Matches($m, '[0-9][0-9.]*')
  if ($hit.Count -ge 2) {
    $have = $hit[$hit.Count - 2].Value.TrimEnd('.')
    $need = $hit[$hit.Count - 1].Value.TrimEnd('.')
    return @{ Have = $have; Need = $need }
  }
  return $null
}

# ===========================================================================
# Step 1 - which tools are we allowed to touch
# ===========================================================================

Dr-Say
Dr-Say 'DeepRouter one-click setup'
Dr-Say
Dr-Say "  Server : $DrBaseUrl"
Dr-Say
Dr-Say '  Checking which tools are installed...'

$targets      = @()
$skippedLogin = @()

foreach ($id in $DrToolIds) {
  if (-not $DrTools.Contains($id)) { continue }
  if ($DrOnly.Count -gt 0 -and $DrOnly -notcontains $id) { continue }
  $t = $DrTools[$id]

  $cmd = $null
  try { $cmd = Get-Command $t.Exe -ErrorAction SilentlyContinue } catch { }
  if (-not $cmd) {
    # PRD 0.1 F9: a config directory does not prove a tool is installed -
    # the ChatGPT desktop app writes a full ~/.codex/config.toml and never
    # provides a codex command. Naming the directory is the difference
    # between "you never installed this" and "you installed something else".
    $dir = $null
    if ($id -eq 'codex')      { $dir = Join-Path $env:USERPROFILE '.codex' }
    if ($id -eq 'gemini-cli') { $dir = Join-Path $env:USERPROFILE '.gemini' }
    if ($dir -and (Test-Path $dir)) {
      Dr-None "$($t.Name)  config directory found, but no ``$($t.Exe)`` command - skipped"
    } else {
      Dr-None "$($t.Name)  not installed - skipped"
    }
    continue
  }

  if ($t.Login -and (Test-Path $t.Login) -and ($DrForce -notcontains $id)) {
    # Existence is the whole test. We do not read it: any existing login is
    # one we must not displace, so which kind it is has no use, and a script
    # already holding the user's key has no business parsing their
    # credentials (PRD 4.2.1).
    $skippedLogin += $id
    Dr-Skip "$($t.Name)  already signed in - left alone"
    Dr-Note 'Switching it to DeepRouter would mean paying for both.'
    Dr-Note "To switch anyway: re-run with -Force $id"
    continue
  }

  $targets += $id
  Dr-Ok $t.Name
}

if ($targets.Count -eq 0) {
  Dr-Say
  if ($skippedLogin.Count -gt 0) {
    Dr-Say '  Nothing was changed. Every tool found is already signed in.'
    Dr-Say '  Re-run with -Force <tool> if you want it switched to DeepRouter.'
  } else {
    Dr-Say '  None of the supported tools were found on this machine.'
    Dr-Say
    Dr-Say '  Supported: Claude Code, OpenCode, Codex CLI, Gemini CLI'
    Dr-Say '  Install one, then copy a fresh command from your keys page.'
  }
  Dr-Say
  return
}

# ===========================================================================
# Step 2 - pick a model by calling it
# ===========================================================================

function Invoke-DrProbe {
  param([string]$Proto, [string]$Model, [int]$MaxTokens)
  if ($Proto -eq 'anthropic') {
    $b = @{ model = $Model; max_tokens = $MaxTokens
            messages = @(@{ role = 'user'; content = 'hi' }) } | ConvertTo-Json -Depth 10 -Compress
    return Invoke-DrHttp 'POST' "$DrBaseUrl/v1/messages" $b
  }
  $b = @{ model = $Model; max_tokens = $MaxTokens
          messages = @(@{ role = 'user'; content = 'hi' }) } | ConvertTo-Json -Depth 10 -Compress
  return Invoke-DrHttp 'POST' "$DrBaseUrl/v1/chat/completions" $b
}

# A name test is not enough by itself, but it is the only thing that catches
# the dangerous case: gpt-4o-mini-tts answers a chat request with a 403
# reserve failure, which reads exactly like "top up" for a model that will
# never hold a conversation (PRD 0.1 F19).
function Test-DrChatModel {
  param([string]$Id)
  return ($Id -notmatch 'tts|audio|whisper|image|video|vision-preview|embed|rerank|moderation')
}

function Get-DrRank {
  param([string]$Id)
  if ($Id -match 'nano')   { return 1 }
  if ($Id -match 'mini')   { return 2 }
  if ($Id -match 'flash')  { return 3 }
  if ($Id -match 'haiku')  { return 4 }
  if ($Id -match 'small')  { return 5 }
  if ($Id -match 'turbo')  { return 6 }
  if ($Id -match 'sonnet') { return 8 }
  if ($Id -match 'opus')   { return 9 }
  return 7
}

$script:DrModelList = $null
function Get-DrModels {
  if ($null -ne $script:DrModelList) { return $script:DrModelList }
  $script:DrModelList = @()
  $r = Invoke-DrHttp 'GET' "$DrBaseUrl/v1/models" $null
  if ($r.Code -ne 200) { return $script:DrModelList }
  try {
    $o = $r.Body | ConvertFrom-Json
    foreach ($m in $o.data) {
      $script:DrModelList += @{ Id = [string]$m.id; Types = @($m.supported_endpoint_types) }
    }
  } catch { }
  return $script:DrModelList
}

function Get-DrCandidates {
  param([string]$Proto)
  $out = @()
  foreach ($m in (Get-DrModels)) {
    if (-not $m.Id) { continue }
    if (-not (Test-DrChatModel $m.Id)) { continue }
    # No declared types means the deployment did not say. Trying it beats
    # dropping a model that may well work.
    if ($m.Types -and $m.Types.Count -gt 0 -and ($m.Types -notcontains $Proto)) { continue }
    $out += $m.Id
  }
  return @($out | Sort-Object @{ Expression = { Get-DrRank $_ } }, @{ Expression = { $_ } })
}

$script:DrAuthFailed = $false
$script:DrFunds = $null
$script:DrNetFailed = $false
$chosen = @{}

function Select-DrModel {
  param([string]$Proto, [int]$MaxTokens)

  # deeprouter-auto first. It is the only option that gets smart routing, and
  # whether a deployment supports it changes with the deployment, so asking
  # is the only answer that is not a guess (PRD 4.3 step 1).
  $r = Invoke-DrProbe $Proto 'deeprouter-auto' $MaxTokens
  $c = Get-DrClass $r
  if ($c -eq 'ok') { return 'deeprouter-auto' }
  if ($c -eq 'auth') { $script:DrAuthFailed = $true; return $null }
  if ($c -eq 'network') { $script:DrNetFailed = $true; return $null }

  foreach ($m in (Get-DrCandidates $Proto)) {
    $r = Invoke-DrProbe $Proto $m $MaxTokens
    switch (Get-DrClass $r) {
      'ok'   { return $m }
      'busy' { return $m }   # reachable and permitted, just loaded
      'auth' { $script:DrAuthFailed = $true; return $null }
      # No wire, so no candidate can do better - stop probing.
      'network' { $script:DrNetFailed = $true; return $null }
      'funds' {
        # Keep the figures but keep going - a cheaper candidate may still be
        # affordable, and only running out of them means "top up".
        $f = Get-DrFunds $r
        if ($f) { $script:DrFunds = $f }
      }
    }
  }
  return $null
}

Dr-Say
Dr-Say '  Choosing a model that really works...'

foreach ($id in $targets) {
  $proto = $DrTools[$id].Proto
  if ($chosen.ContainsKey($proto)) { continue }
  $picked = Select-DrModel $proto $DrTools[$id].MaxTokens
  if ($picked) {
    $chosen[$proto] = $picked
    if ($picked -eq 'deeprouter-auto') { Dr-Ok "${proto}: $picked (smart routing)" }
    else { Dr-Ok "${proto}: $picked" }
  } else {
    if ($script:DrNetFailed) { Dr-Bad "${proto}: could not reach the server" }
    else { Dr-Bad "${proto}: no model on this account could answer" }
  }
}

if ($script:DrNetFailed) {
  Dr-Say
  Dr-Bad "Cannot reach DeepRouter at $DrBaseUrl."
  Dr-Note 'Check your network connection, then copy a fresh command and try again.'
  Dr-Say
  return
}

if ($script:DrAuthFailed) {
  Dr-Say
  Dr-Bad 'That key was rejected.'
  Dr-Note 'Generate a new key on your API keys page and copy a fresh command.'
  Dr-Say
  return
}

# Codex and Gemini never get deeprouter-auto: their request bodies use `input`
# and `contents`, the router reads neither, and it falls back to a default
# model silently rather than failing (PRD 4.3).
function Get-DrModelFor {
  param([string]$Id)
  $proto = $DrTools[$Id].Proto
  if (-not $chosen.ContainsKey($proto)) { return $null }
  $m = $chosen[$proto]
  if ($m -eq 'deeprouter-auto' -and ($Id -eq 'codex' -or $Id -eq 'gemini-cli')) {
    $alt = @(Get-DrCandidates $proto)
    if ($alt.Count -gt 0) { return $alt[0] }
  }
  return $m
}

$configurable = @($targets | Where-Object { Get-DrModelFor $_ })
if ($configurable.Count -eq 0) {
  Dr-Say
  Dr-Bad 'No usable model was found for the tools you selected.'
  if ($script:DrFunds) {
    Dr-Note "Your balance is `$$($script:DrFunds.Have) and the cheapest model needs `$$($script:DrFunds.Need) held aside."
    Dr-Note "Top up here: $DrBaseUrl/topup"
  } else {
    Dr-Note 'Check that your key is allowed to use at least one chat model.'
  }
  Dr-Say
  return
}

# ===========================================================================
# Step 3 - back up, then merge
# ===========================================================================

if (-not (Test-Path $DrDir)) { New-Item -ItemType Directory -Path $DrDir -Force | Out-Null }

$manifestTools = @()
$envVars       = [ordered]@{}
$wrote         = @()
$failed        = @()
$codexProfile  = $false

# Carry the first install forward. A second run must not overwrite
# original_backup - the earliest copy is the only one that is really the
# user's own state (PRD 4.6).
$prior = $null
if (Test-Path $DrManifest) {
  try { $prior = (Get-Content -Raw -Path $DrManifest) | ConvertFrom-Json } catch { $prior = $null }
}
function Get-DrPrior {
  param([string]$Name)
  if ($null -eq $prior -or $null -eq $prior.tools) { return $null }
  foreach ($e in $prior.tools) { if ($e.name -eq $Name) { return $e } }
  return $null
}

function New-DrBackup {
  param([string]$File)
  if (-not (Test-Path $File)) { return $null }
  # $PID as well as the stamp: DrStamp has one-second resolution, so two runs
  # in the same second named their backups identically and the second copy
  # destroyed the first run's copy of the user's true original - while the
  # manifest kept pointing at the now-clobbered path. Same fix as setup.sh.
  $bak = "$File.bak-$DrStamp-$PID"
  Copy-Item -LiteralPath $File -Destination $bak -Force
  return $bak
}

function Add-DrRecord {
  param([string]$Id, [string]$File, [bool]$PreExisting, $Backup)
  $p = Get-DrPrior $Id
  if ($null -ne $p) { $PreExisting = [bool]$p.pre_existing; $Backup = $p.original_backup }
  $script:manifestTools += @{
    name = $Id; file = $File; pre_existing = $PreExisting; original_backup = $Backup
  }
}

# --- JSON merge -------------------------------------------------------------
# ConvertFrom-Json / ConvertTo-Json is the Windows counterpart of setup.sh's
# awk parser: same contract, same guarantees. A file that does not parse
# leaves its tool skipped and the file untouched, which is the required
# behaviour for a corrupt config (PRD 4.3).

function Set-DrJsonPath {
  param($Root, [string[]]$Path, $Value)
  $cur = $Root
  for ($i = 0; $i -lt $Path.Count - 1; $i++) {
    $seg = $Path[$i]
    $next = $null
    if ($cur.PSObject.Properties.Name -contains $seg) { $next = $cur.$seg }
    if ($null -eq $next -or -not ($next -is [PSCustomObject])) {
      $next = New-Object PSObject
      if ($cur.PSObject.Properties.Name -contains $seg) { $cur.$seg = $next }
      else { $cur | Add-Member -NotePropertyName $seg -NotePropertyValue $next }
    }
    $cur = $next
  }
  $leaf = $Path[$Path.Count - 1]
  if ($cur.PSObject.Properties.Name -contains $leaf) { $cur.$leaf = $Value }
  else { $cur | Add-Member -NotePropertyName $leaf -NotePropertyValue $Value }
}

function Write-DrJson {
  param([string]$Path, $Object)
  # Depth 100 because 5.1 defaults to 2 and silently flattens anything
  # deeper into a string. No BOM, so other tools can read it (PRD 0.1 F6).
  $text = $Object | ConvertTo-Json -Depth 100
  [System.IO.File]::WriteAllText($Path, $text, (New-Object System.Text.UTF8Encoding($false)))
}

# dr_setup_* - one per tool, mirroring setup.sh function for function.

function Set-DrClaudeCode {
  # PRD 0.1 F1: the env block inside .claude\settings.json does not work, it
  # still asks you to log in. Only real environment variables do - so we
  # never touch that file, and the user's permissions, hooks and theme are
  # out of reach by construction.
  $m = Get-DrModelFor 'claude-code'
  $script:envVars['ANTHROPIC_BASE_URL']         = $DrBaseUrl
  $script:envVars['ANTHROPIC_AUTH_TOKEN']       = $DrApiKey
  $script:envVars['ANTHROPIC_MODEL']            = $m
  $script:envVars['ANTHROPIC_SMALL_FAST_MODEL'] = $m
  Add-DrRecord 'claude-code' '' $true $null
  Dr-Ok 'Claude Code    environment variables'
  return $true
}

function Get-DrOpenCodeConfig {
  try {
    $out = & opencode debug paths 2>$null
    foreach ($line in $out) {
      if ($line -match '([A-Za-z]:[\\/][^\s"]*\.json)') { return $Matches[1] }
    }
  } catch { }
  if ($env:XDG_CONFIG_HOME) { return (Join-Path $env:XDG_CONFIG_HOME 'opencode\opencode.json') }
  return (Join-Path $env:USERPROFILE '.config\opencode\opencode.json')
}

function Set-DrOpenCode {
  $file = Get-DrOpenCodeConfig
  $model = Get-DrModelFor 'opencode'
  $pre = $false
  $bak = $null

  $dir = Split-Path -Parent $file
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }

  $doc = $null
  if (Test-Path $file) {
    $pre = $true
    try { $doc = (Get-Content -Raw -Path $file) | ConvertFrom-Json } catch {
      Dr-Bad "OpenCode       $file is not valid JSON - left untouched"
      Dr-Note 'Fix or move that file, then copy a fresh command from your keys page.'
      return $false
    }
    if ($null -eq $doc) { $doc = New-Object PSObject }
    $bak = New-DrBackup $file
  } else {
    $doc = New-Object PSObject
  }

  $provider = [PSCustomObject]@{
    npm     = '@ai-sdk/openai-compatible'
    name    = 'DeepRouter'
    options = [PSCustomObject]@{ baseURL = "$DrBaseUrl/v1"; apiKey = $DrApiKey }
    models  = New-Object PSObject
  }
  $provider.models | Add-Member -NotePropertyName $model `
    -NotePropertyValue ([PSCustomObject]@{ name = 'DeepRouter' })

  try {
    Set-DrJsonPath $doc @('provider', 'deeprouter') $provider
    # Top-level default too. With only a provider entry, OpenCode still
    # starts on whatever model its own catalog picks (seen live
    # 2026-08-28: Nano Banana Pro, demanding a Google key). Overwrites a
    # user-set default on purpose - pointing the tool at DeepRouter is
    # what running this means.
    Set-DrJsonPath $doc @('model') "deeprouter/$model"
    Write-DrJson $file $doc
  } catch {
    Dr-Bad "OpenCode       could not rewrite $file - left untouched"
    return $false
  }

  Add-DrRecord 'opencode' $file $pre $bak
  if ($bak) { Dr-Ok "OpenCode       $file (backed up)" } else { Dr-Ok "OpenCode       $file" }
  return $true
}

function Get-DrCodexBody {
  param([string]$Model)
  $lines = @(
    "model = `"$Model`"",
    'model_provider = "deeprouter"',
    '',
    '[model_providers.deeprouter]',
    'name = "DeepRouter"',
    "base_url = `"$DrBaseUrl/v1`"",
    'env_key = "DEEPROUTER_API_KEY"',
    # wire_api = "chat" was removed in v0.149.1 and now stops Codex loading
    # its config at all (PRD 0.1 F13).
    'wire_api = "responses"'
  )
  return ($lines -join "`r`n") + "`r`n"
}

function Set-DrCodex {
  $main = Join-Path $env:USERPROFILE '.codex\config.toml'
  $model = Get-DrModelFor 'codex'
  $dir = Split-Path -Parent $main
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }

  $script:envVars['DEEPROUTER_API_KEY'] = $DrApiKey

  try {
    if (Test-Path $main) {
      # PRD Q7, measured: someone who already configured Codex has an
      # opinion about their default model. Writing a separate profile file
      # respects that and removes the need to edit TOML at all.
      $file = Join-Path $env:USERPROFILE '.codex\deeprouter.config.toml'
      $script:codexProfile = $true
      $pre = Test-Path $file
      $bak = $null
      if ($pre) { $bak = New-DrBackup $file }
      # The same content as the standalone case, in a file Codex layers on top
      # when asked. `--profile <name>` means "layer $CODEX_HOME/<name>.config.toml
      # over the base config", so these keys belong at the TOP level - wrapping
      # them in a [profiles.deeprouter] section buries them in a nested profile
      # nothing activates, and Codex quietly keeps using the base provider.
      # Measured 2026-08-28 on codex-cli 0.149.1: that one header line was the
      # difference between `provider: deeprouter` and `provider: openai`.
      [System.IO.File]::WriteAllText($file, (Get-DrCodexBody $model),
        (New-Object System.Text.UTF8Encoding($false)))
      Add-DrRecord 'codex' $file $pre $bak
      Dr-Ok "Codex CLI      $file (your config.toml was not touched)"
    } else {
      [System.IO.File]::WriteAllText($main, (Get-DrCodexBody $model),
        (New-Object System.Text.UTF8Encoding($false)))
      Add-DrRecord 'codex' $main $false $null
      Dr-Ok "Codex CLI      $main"
    }
  } catch {
    Dr-Bad 'Codex CLI      cannot write its configuration'
    return $false
  }
  return $true
}

function Set-DrGemini {
  # PRD 0.1 F10: without security.auth.selectedType the CLI refuses before
  # sending anything - and it is a nested key, the old flat one does nothing.
  # F12: the model name lives here too, GEMINI_MODEL is never read.
  $file = Join-Path $env:USERPROFILE '.gemini\settings.json'
  $model = Get-DrModelFor 'gemini-cli'
  $pre = $false
  $bak = $null

  $dir = Split-Path -Parent $file
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }

  $doc = $null
  if (Test-Path $file) {
    $pre = $true
    try { $doc = (Get-Content -Raw -Path $file) | ConvertFrom-Json } catch {
      Dr-Bad "Gemini CLI     $file is not valid JSON - left untouched"
      Dr-Note 'Fix or move that file, then copy a fresh command from your keys page.'
      return $false
    }
    if ($null -eq $doc) { $doc = New-Object PSObject }
    $bak = New-DrBackup $file
  } else {
    $doc = New-Object PSObject
  }

  try {
    Set-DrJsonPath $doc @('security', 'auth', 'selectedType') 'gemini-api-key'
    Set-DrJsonPath $doc @('model', 'name') $model
    Write-DrJson $file $doc
  } catch {
    Dr-Bad "Gemini CLI     could not rewrite $file - left untouched"
    return $false
  }

  $script:envVars['GEMINI_API_KEY'] = $DrApiKey
  # Never with /v1beta - the CLI appends it, and the doubled path comes back
  # as an error that says nothing about the cause (PRD 0.1 F11).
  $script:envVars['GOOGLE_GEMINI_BASE_URL'] = $DrBaseUrl
  Add-DrRecord 'gemini-cli' $file $pre $bak
  if ($bak) { Dr-Ok "Gemini CLI     $file (backed up)" } else { Dr-Ok "Gemini CLI     $file" }
  return $true
}

Dr-Say
Dr-Say '  Writing configuration...'

foreach ($id in $configurable) {
  $okFlag = $false
  switch ($id) {
    'claude-code' { $okFlag = Set-DrClaudeCode }
    'opencode'    { $okFlag = Set-DrOpenCode }
    'codex'       { $okFlag = Set-DrCodex }
    'gemini-cli'  { $okFlag = Set-DrGemini }
  }
  if ($okFlag) { $wrote += $id } else { $failed += $id }
}

# ===========================================================================
# Step 4 - environment variables, in the registry
# ===========================================================================
# This is where the two platforms genuinely differ (PRD 3 D2, measured).
# Mirroring the POSIX side with $PROFILE plus a sourced .ps1 fails twice over
# under the default Restricted execution policy: the variables never get set,
# AND the user sees a red error in every new PowerShell window from then on,
# pointing at a file we wrote. SetEnvironmentVariable is a .NET call, so the
# policy is irrelevant to it - and it reaches cmd and GUI apps too.
#
# Measured alongside it: values keep their % signs (stored REG_SZ, not
# REG_EXPAND_SZ, so a key containing % is not silently rewritten - V6), and
# no elevation is required (V7).

if ($envVars.Count -gt 0) {
  # Say this before doing it. Each SetEnvironmentVariable(..., "User")
  # broadcasts WM_SETTINGCHANGE and blocks until the windows on the desktop
  # answer - measured at about seven seconds each on a normal machine, so
  # seven variables is close to a minute of an otherwise silent screen right
  # after "Writing configuration...". Users kill things that look hung.
  #
  # Writing the registry directly and broadcasting once at the end does cut
  # that to about seven seconds, and it was tried: it also produced writes
  # that intermittently did not take, with no error. This is the mechanism
  # PRD 0.1 V5-V8 actually verified end to end, so it stays - a slow install
  # beats a fast one that silently configures nothing.
  Dr-Say "                 (this part takes up to a minute - Windows tells every"
  Dr-Say "                  running program about each variable, and waits)"
  $failedVars = @()
  foreach ($k in $envVars.Keys) {
    try {
      [Environment]::SetEnvironmentVariable($k, $envVars[$k], 'User')
      # So a tool started from this same session can already see it. The new
      # window is still required for anything launched later.
      Set-Item -Path "env:$k" -Value $envVars[$k] -ErrorAction SilentlyContinue
    } catch { $failedVars += $k }
  }
  if ($failedVars.Count -eq 0) {
    Dr-Ok "Environment    $($envVars.Count) variable(s) set for your user account"
  } else {
    Dr-Bad "Environment    could not set: $($failedVars -join ', ')"
  }
}

# ===========================================================================
# Step 5 - the manifest
# ===========================================================================
# On Windows this is the only record that exists. Nothing is written to disk
# for the environment variables, so without the names listed here uninstall
# has nothing to work from (PRD 4.6).

$manifest = [ordered]@{
  installed_at = $DrNow
  base_url     = $DrBaseUrl
  tools        = @($manifestTools | ForEach-Object {
                    [ordered]@{ name = $_.name; file = $_.file
                                pre_existing = $_.pre_existing
                                original_backup = $_.original_backup } })
  # Windows has no shell file at all - the variables go to the registry - but
  # the field stays so both platforms write one manifest schema.
  shell        = [ordered]@{ file = ''; line = ''; pre_existing = $false }
  env_file     = ''
  env_vars     = @($envVars.Keys)
}
try {
  Write-DrJson $DrManifest ([PSCustomObject]$manifest)
} catch {
  Dr-Bad "Could not write $DrManifest - uninstall will not know what to undo."
}

# ===========================================================================
# Step 6 - prove it works, one protocol at a time
# ===========================================================================

function Invoke-DrVerify {
  param([string]$Kind, [string]$Model)
  switch ($Kind) {
    'anthropic' {
      $b = @{ model = $Model; max_tokens = 16
              messages = @(@{ role = 'user'; content = 'hi' }) } | ConvertTo-Json -Depth 10 -Compress
      return Invoke-DrHttp 'POST' "$DrBaseUrl/v1/messages" $b }
    'openai' {
      $b = @{ model = $Model; max_tokens = 16
              messages = @(@{ role = 'user'; content = 'hi' }) } | ConvertTo-Json -Depth 10 -Compress
      return Invoke-DrHttp 'POST' "$DrBaseUrl/v1/chat/completions" $b }
    'responses' {
      $b = @{ model = $Model; max_output_tokens = 16; input = 'hi' } | ConvertTo-Json -Depth 10 -Compress
      return Invoke-DrHttp 'POST' "$DrBaseUrl/v1/responses" $b }
    'gemini' {
      $b = @{ contents = @(@{ role = 'user'; parts = @(@{ text = 'hi' }) }) } | ConvertTo-Json -Depth 10 -Compress
      return Invoke-DrHttp 'POST' "$DrBaseUrl/v1beta/models/${Model}:generateContent" $b }
  }
}

$verifyLabels = @{
  anthropic = 'Anthropic        '; openai = 'OpenAI           '
  responses = 'OpenAI Responses '; gemini = 'Google Gemini    '
}

$verifyOk = 0
$verifyBad = 0

if ($wrote.Count -gt 0) {
  Dr-Say
  Dr-Say '  Verifying...'
  foreach ($id in $wrote) {
    $kind = $DrTools[$id].Verify
    $lbl = $verifyLabels[$kind]
    $name = $DrTools[$id].Name
    $r = Invoke-DrVerify $kind (Get-DrModelFor $id)
    switch (Get-DrClass $r) {
      'ok'   { $verifyOk++; Dr-Ok "$lbl works   ($name)" }
      'busy' {
        $verifyOk++
        Dr-Skip "$lbl the model is busy right now ($name)"
        Dr-Note 'Your configuration is written and correct - just try again in a moment.' }
      'funds' {
        $verifyBad++
        Dr-Bad "$lbl not enough balance ($name)"
        $f = Get-DrFunds $r
        if ($f) { Dr-Note "You have `$$($f.Have); this model needs `$$($f.Need) held aside per request." }
        Dr-Note "Top up here: $DrBaseUrl/topup" }
      'auth' {
        $verifyBad++
        Dr-Bad "$lbl the key was rejected ($name)"
        Dr-Note 'Generate a new key on your API keys page.' }
      'network' {
        $verifyBad++
        Dr-Bad "$lbl could not reach DeepRouter ($name)"
        Dr-Note 'Check your network and run the command again.' }
      default {
        $verifyBad++
        Dr-Bad "$lbl did not work ($name)"
        $m = Get-DrErrMessage $r
        if ($m) { Dr-Note "The gateway said: $m" } }
    }
  }
}

# ===========================================================================
# Step 7 - the report
# ===========================================================================

Dr-Say
Dr-Say '  --------------------------------------------------------------'
if ($wrote.Count -eq 0) { Dr-Say '  Nothing was configured.' }
else { Dr-Say "  Done. $($wrote.Count) tool(s) configured." }

if ($failed.Count -gt 0) {
  Dr-Say
  foreach ($id in $failed) { Dr-Note "$($DrTools[$id].Name) was left as it was - see the message above." }
}

# Every variable we set went to the registry, and Windows gives a new process
# the environment block of its parent rather than re-reading the registry.
# So it is genuinely a new window, not a shell started from this one (PRD 3 D2).
$needsRestart = @($wrote | Where-Object { $_ -ne 'opencode' })

# One table, one row per configured tool: the exact thing to type, and when.
# This exists because a real user typed plain `codex` after installing, met the
# ChatGPT login screen, and concluded setup had failed - the old prose hint was
# printed and still not seen (2026-08-28).
if ($wrote.Count -gt 0) {
  Dr-Say
  Dr-Say '  How to start each tool:'
  Dr-Say
  foreach ($id in $wrote) {
    switch ($id) {
      'opencode'    { Dr-Say '    OpenCode      type: opencode                     works right now' }
      'claude-code' { Dr-Say '    Claude Code   type: claude                       new terminal first' }
      'gemini-cli'  { Dr-Say '    Gemini CLI    type: gemini                       new terminal first' }
      'codex'       {
        if ($codexProfile) { Dr-Say '    Codex CLI     type: codex --profile deeprouter   new terminal first' }
        else               { Dr-Say '    Codex CLI     type: codex                        new terminal first' }
      }
    }
  }
  if ($needsRestart.Count -gt 0) {
    Dr-Say
    Dr-Say '  ! "New terminal first" means: Close this window completely and'
    Dr-Say '    open a new one - starting another terminal from inside this'
    Dr-Say '    one is not enough.'
  }
}

# The one start command that fails in a misleading way gets shouted. Plain
# codex does not error - it shows the ChatGPT login screen, which reads as
# "setup did not work" to exactly the person this product is for.
if ($codexProfile) {
  Dr-Say
  Dr-Say '  !!! CODEX - READ THIS !!!'
  Dr-Say '  !!! Typing plain codex will STILL show the ChatGPT login screen.'
  Dr-Say '  !!! You already had Codex settings, so they were left untouched'
  Dr-Say '  !!! and DeepRouter went into a separate profile.'
  Dr-Say '  !!! To use DeepRouter, ALWAYS start it as:'
  Dr-Say '  !!!'
  Dr-Say '  !!!     codex --profile deeprouter'
}

if ($wrote -contains 'gemini-cli') {
  Dr-Say
  Dr-Say '  !!! GEMINI - READ THIS !!!'
  Dr-Say '  !!! The first time gemini starts it may ask how you want to'
  Dr-Say '  !!! sign in. Pick the "API key" option - "Login with Google"'
  Dr-Say '  !!! would switch it away from DeepRouter.'
}

if ($wrote -contains 'claude-code') {
  Dr-Say
  Dr-Say '  ! The first time you run claude it asks "Is this a project you'
  Dr-Say '    trust?" - press Enter. You do not need an Anthropic account.'
  # The one thing this setup takes away. Not saying it means the user finds
  # out later and cannot connect it to us (PRD 0.1 F15).
  Dr-Say '  ! Claude Code will show "claude.ai connectors are disabled" at the'
  Dr-Say '    bottom. That is expected while it runs through DeepRouter, and'
  Dr-Say '    uninstalling brings them back.'
}

Dr-Say
Dr-Say "  Undo anytime:  irm $DrBaseUrl/uninstall | iex"
Dr-Say
