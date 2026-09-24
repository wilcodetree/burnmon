# Starts burnmon-dev.exe with its own dev-only eval channel open
# (BURNMON_DEV_UICHECK=1, never set by release builds or the README), runs
# one or more tools\uicheck "d*" checks against the real running window,
# then stops the exe. Mirrors scripts\uicheck.ps1's own w-mode flow one
# exe, one port (9334, not 9333) and one window title ("BurnMon Dev", not
# "BurnMon") over; called by uicheck.ps1 itself when any target check
# starts with "d", not normally invoked directly.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$exe = Join-Path (Get-Location) "burnmon-dev.exe"
if (-not (Test-Path $exe)) {
    Write-Host "burnmon-dev.exe not found; run .\build.ps1 first."
    exit 1
}

$targetChecks = $args
if ($targetChecks.Count -eq 0) {
    $targetChecks = @("d0", "d1", "d2", "d3")
}

# Tracked by byte offset, not line count: setupLog (app.go) rotates the log
# to .1 once it passes 512KB, at the START of a run, before this run's own
# "starting" line lands. A line-count baseline taken before launch would
# then be counted against the freshly-rotated (small) file, so
# Select-Object -Skip would skip every line the new run ever wrote and the
# "stage BuildPayload" wait below would always time out (found by review,
# 2026-09-24, a latent bug scripts\uicheck.ps1's own equivalent wait
# already carried; fixed here, not there, since that file is
# burnmon.exe's, out of this phase's own scope). A byte offset survives
# rotation cleanly: if the file is now shorter than the recorded offset, it
# was rotated, so read from 0 instead.
$logPath = "$env:LOCALAPPDATA\burnmon\burnmon-dev-app.log"
$logBytesBefore = 0
if (Test-Path $logPath) { $logBytesBefore = (Get-Item $logPath).Length }

$env:BURNMON_DEV_UICHECK = "1"
$proc = Start-Process -FilePath $exe -PassThru
try {
    # Poll the eval channel instead of a fixed sleep: it appears only once
    # the window and its bdevUICheckResult binding are actually up. Measured
    # against the real store on this laptop (2026-09-24): the port did not
    # open until close to the startup backfill finishing, ~80s, well past
    # burnmon.exe's own 20s budget for the equivalent wait in uicheck.ps1,
    # so this one is more generous.
    $deadline = (Get-Date).AddSeconds(120)
    $ready = $false
    while ((Get-Date) -lt $deadline) {
        try {
            $client = New-Object System.Net.Sockets.TcpClient
            $client.Connect("127.0.0.1", 9334)
            $client.Close()
            $ready = $true
            break
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $ready) {
        Write-Host "Dev eval port 9334 never came up. Either BURNMON_DEV_UICHECK is not being read, or the window failed to start; check %LOCALAPPDATA%\burnmon\burnmon-dev-app.log."
        exit 1
    }
    # Wait for the startup backfill's own last stage line (internal/dataset's
    # Collect logs "stage BuildPayload" last; app.go's startInitialCollect
    # only logs on error, so this is the closest available "it's done"
    # signal), same reasoning as uicheck.ps1's own wait for burnmon.exe's
    # "stage Collect (total)" line: an eval sent before this finishes can
    # otherwise time out waiting for the UI thread mid-backfill.
    $collectDeadline = (Get-Date).AddSeconds(90)
    $collectDone = $false
    while ((Get-Date) -lt $collectDeadline) {
        if (Test-Path $logPath) {
            # setupLog (app.go) has the file open for append the whole time
            # this exe runs; [System.IO.File]::ReadAllBytes opens with
            # exclusive-enough sharing to collide with that. An explicit
            # FileStream with FileShare.ReadWrite reads alongside it instead.
            $stream = [System.IO.File]::Open($logPath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
            try {
                $bytes = New-Object byte[] $stream.Length
                [void]$stream.Read($bytes, 0, $bytes.Length)
            } finally {
                $stream.Close()
            }
            $offset = if ($bytes.Length -lt $logBytesBefore) { 0 } else { $logBytesBefore }
            $newText = [System.Text.Encoding]::UTF8.GetString($bytes, $offset, $bytes.Length - $offset)
            if ($newText -match "stage BuildPayload") {
                $collectDone = $true
                break
            }
        }
        Start-Sleep -Seconds 2
    }
    if (-not $collectDone) {
        Write-Host "Startup backfill never finished within 90s; checks would likely time out anyway. Check %LOCALAPPDATA%\burnmon\burnmon-dev-app.log."
        exit 1
    }

    Push-Location tools\uicheck
    try {
        foreach ($check in $targetChecks) {
            Write-Host "--- uicheck $check ---"
            go run . $check
            if ($LASTEXITCODE -ne 0) {
                Write-Host "uicheck $check FAILED"
                exit $LASTEXITCODE
            }
        }
    } finally {
        Pop-Location
    }
    Write-Host "uicheck: all checks passed"
} finally {
    Remove-Item Env:\BURNMON_DEV_UICHECK -ErrorAction SilentlyContinue
    if (-not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force
    }
}
