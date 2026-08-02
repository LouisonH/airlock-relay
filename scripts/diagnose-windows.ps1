param(
  [switch]$SidecarOnly
)

# Airlock Windows diagnostic helper. Run with:
#   powershell -ExecutionPolicy Bypass -File scripts\diagnose-windows.ps1
$ErrorActionPreference = "Continue"

Write-Host "=== 1. System ==="
Get-ComputerInfo | Select-Object OsName, OsArchitecture, WindowsVersion, WindowsBuildLabEx | Format-List
Write-Host ("PROCESSOR_ARCHITECTURE: " + $env:PROCESSOR_ARCHITECTURE)
Write-Host ("64-bit OS: " + [Environment]::Is64BitOperatingSystem)

Write-Host "`n=== 2. Installed Airlock files ==="
$candidates = @(
  "C:\Program Files\Airlock",
  (Join-Path $env:LOCALAPPDATA "Airlock")
)
foreach ($d in $candidates) {
  if (Test-Path $d) {
    Write-Host "--- $d"
    Get-ChildItem $d -Recurse -File | Select-Object FullName, Length, LastWriteTime | Format-Table -AutoSize
  }
}

Write-Host "`n=== 3. Core startup log ==="
$log = Join-Path $env:APPDATA "io.airlock.relay\airlockd-startup.log"
if (Test-Path $log) {
  Get-Content $log
} else {
  Write-Host "(no startup log at $log)"
}

Write-Host "`n=== 4. App data directory ==="
$cfg = Join-Path $env:APPDATA "io.airlock.relay"
if (Test-Path $cfg) {
  Get-ChildItem $cfg -Force | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize
} else {
  Write-Host "(no app data directory at $cfg)"
}

if ($SidecarOnly) { exit 0 }

Write-Host "`n=== 5. WebView2 runtime ==="
$keys = @(
  "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
  "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
)
$found = $false
foreach ($key in $keys) {
  $item = Get-ItemProperty $key -ErrorAction SilentlyContinue
  if ($item) { Write-Host ("WebView2 runtime version: " + $item.pv); $found = $true }
}
if (-not $found) {
  $wvDir = "C:\Program Files\Microsoft\EdgeWebView\Application"
  if (Test-Path $wvDir) {
    Write-Host ("WebView2 folder versions: " + ((Get-ChildItem $wvDir -Directory | Select-Object -ExpandProperty Name) -join ", "))
  } else {
    Write-Host "WebView2 runtime NOT found. Install WebView2 Runtime (ARM64) from Microsoft."
  }
}

Write-Host "`n=== 6. Direct sidecar test ==="
$exe = $null
foreach ($d in $candidates) {
  $p = Join-Path $d "airlockd.exe"
  if (Test-Path $p) { $exe = $p; break }
}
if ($exe) {
  Write-Host ("Testing: " + $exe)
  $tmp = Join-Path $env:TEMP "airlock-diag"
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  $token = "diagnostic-token-0000000000000000"
  $output = $token | & $exe --control-token-stdin --control-dir $tmp --listen 127.0.0.1:4768 --ssh-listen 127.0.0.1:4770 --network-scope loopback --secret-store local_file 2>&1
  Write-Host "--- sidecar output (first 20 lines) ---"
  $output | Select-Object -First 20
  Write-Host ("exit code: " + $LASTEXITCODE)
} else {
  Write-Host "airlockd.exe not found in Program Files or LOCALAPPDATA."
}

Write-Host "`n=== 7. Listener ports ==="
Get-NetTCPConnection -LocalPort 4768,4770 -ErrorAction SilentlyContinue |
  Select-Object LocalAddress, LocalPort, State, OwningProcess | Format-Table -AutoSize

Write-Host "`nDone. Please paste this output together with a screenshot of the Airlock window."
