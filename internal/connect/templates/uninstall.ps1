# DeepRouter uninstall (Windows PowerShell 5.1+).
#
# Puts this machine back the way it was before one-click setup ran. It needs no
# token and no key, which is why it is a fixed address that always works:
#
#   irm <base>/uninstall | iex
#
# Everything it does comes from %USERPROFILE%\.deeprouter\installed.json.
# Without that file it does nothing at all - and on Windows there is nothing
# else it could fall back on: setup writes no file for the environment
# variables, so the manifest's list of names is the only record that they were
# ever set (PRD 4.6).
#
# ASCII only, same reason as setup.ps1 (PRD 0.1 F6).

$ErrorActionPreference = 'Stop'

$DrDir      = Join-Path $env:USERPROFILE '.deeprouter'
$DrManifest = Join-Path $DrDir 'installed.json'

function Dr-Say  { param([string]$m = '') Write-Host $m }
function Dr-Ok   { param([string]$m) Write-Host "  [ ok ] $m" }
function Dr-None { param([string]$m) Write-Host "  [ -- ] $m" }
function Dr-Bad  { param([string]$m) Write-Host "  [fail] $m" }
function Dr-Note { param([string]$m) Write-Host "         $m" }

Dr-Say
Dr-Say 'DeepRouter uninstall'
Dr-Say

if (-not (Test-Path $DrManifest)) {
  Dr-Say '  No DeepRouter setup was found on this machine.'
  Dr-Say '  Nothing was changed.'
  Dr-Say
  Dr-Say "  (Looked for $DrManifest. Without it there is no record of what"
  Dr-Say '   to undo, and guessing would risk your own configuration.)'
  Dr-Say
  return
}

$m = $null
try { $m = (Get-Content -Raw -Path $DrManifest) | ConvertFrom-Json } catch { $m = $null }
if ($null -eq $m) {
  Dr-Bad "$DrManifest is unreadable."
  Dr-Note 'Nothing was changed. Delete that file by hand if you want to start over.'
  Dr-Say
  return
}

# --- 1. tool configuration files --------------------------------------------
# pre_existing decides between two opposite actions, and getting it backwards
# is destructive either way: restoring a file we invented leaves a stale one
# behind, deleting one the user already had loses their settings.

Dr-Say '  Restoring configuration...'

foreach ($t in @($m.tools)) {
  if (-not $t.file) { continue }   # Claude Code was environment variables only
  if ($t.pre_existing) {
    if ($t.original_backup -and (Test-Path $t.original_backup)) {
      try {
        Copy-Item -LiteralPath $t.original_backup -Destination $t.file -Force
        Remove-Item -LiteralPath $t.original_backup -Force -ErrorAction SilentlyContinue
        Dr-Ok "$($t.name)  restored from the copy taken before the first install"
      } catch {
        Dr-Bad "$($t.name)  could not restore $($t.file)"
        Dr-Note "Your original is still at $($t.original_backup)"
      }
    } else {
      Dr-Bad "$($t.name)  the original copy is missing - $($t.file) left as it is"
      Dr-Note 'Edit it by hand to remove the DeepRouter entries.'
    }
  } else {
    if (Test-Path $t.file) {
      try {
        Remove-Item -LiteralPath $t.file -Force
        Dr-Ok "$($t.name)  removed $($t.file) (we created it)"
      } catch { Dr-Bad "$($t.name)  could not remove $($t.file)" }
    } else {
      Dr-None "$($t.name)  $($t.file) is already gone"
    }
  }
}

# --- 2. environment variables, by name --------------------------------------
# By name, one at a time. HKCU\Environment also holds Path, TEMP and whatever
# else the user has put there, so clearing the key wholesale would take their
# settings with ours. Setting a value to $null is what removes the entry
# cleanly - measured, along with the fact that the user's own variables come
# through untouched (PRD 0.1 V8).

$names = @($m.env_vars)
if ($names.Count -gt 0) {
  $removed = 0
  $failed = @()
  foreach ($n in $names) {
    if (-not $n) { continue }
    try {
      [Environment]::SetEnvironmentVariable($n, $null, 'User')
      Remove-Item -Path "env:$n" -ErrorAction SilentlyContinue
      $removed++
    } catch { $failed += $n }
  }
  if ($failed.Count -eq 0) {
    Dr-Ok "Environment   $removed variable(s) removed; your own were not touched"
  } else {
    Dr-Bad "Environment   could not remove: $($failed -join ', ')"
  }
}

# --- 3. our own directory ----------------------------------------------------

if (Test-Path $DrDir) {
  try { Remove-Item -LiteralPath $DrDir -Recurse -Force; Dr-Ok "Removed $DrDir" }
  catch { Dr-Bad "Could not remove $DrDir" }
}

Dr-Say
Dr-Say '  --------------------------------------------------------------'
Dr-Say '  Done. This machine is back to how it was before setup ran.'
Dr-Say
Dr-Say '  ! Open a new terminal - this one still has the old variables in'
Dr-Say '    its environment until you do.'
Dr-Say
