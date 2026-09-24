# Starts burnmon.exe with its dev-only eval channel open (BURNMON_UICHECK=1,
# never set by release builds or the README), runs one or more
# tools\uicheck checks against the real running window, then stops the exe.
# Screenshots and clicks/keys go straight from tools\uicheck to the window
# via Win32 (PrintWindow, SendInput); only DOM reads/evals go through the
# BURNMON_UICHECK channel, since only burnmon.exe itself can run JS in its
# own page. Run from the repo root in PowerShell:
#   .\scripts\uicheck.ps1                 # runs every registered check
#   .\scripts\uicheck.ps1 w0 w1           # runs only the named checks
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$exe = Join-Path (Get-Location) "burnmon.exe"
if (-not (Test-Path $exe)) {
    Write-Host "burnmon.exe not found; run .\build.ps1 first."
    exit 1
}

$targetChecks = $args
if ($targetChecks.Count -eq 0) {
    $targetChecks = @("w0", "w1", "w2", "w3", "w4", "w5", "w6", "w7", "w8")
}
# Self-managed checks (tools\uicheck\main.go's own selfManaged map: w1 tests
# startup itself) start and stop their own burnmon.exe instance, so each must
# run before anything else here starts one, outside this script's own
# Start-Process/wait-for-port flow.
$selfManagedChecks = @("w1")
foreach ($sm in $selfManagedChecks) {
    if ($targetChecks -contains $sm) {
        Push-Location tools\uicheck
        try {
            Write-Host "--- uicheck $sm (self-managed) ---"
            go run . $sm
            if ($LASTEXITCODE -ne 0) {
                Write-Host "uicheck $sm FAILED"
                exit $LASTEXITCODE
            }
        } finally {
            Pop-Location
        }
        $targetChecks = $targetChecks | Where-Object { $_ -ne $sm }
    }
}
if ($targetChecks.Count -eq 0) {
    Write-Host "uicheck: all checks passed"
    exit 0
}

$logPath = "$env:LOCALAPPDATA\burnmon\burnmon-app.log"
$logLinesBefore = 0
if (Test-Path $logPath) { $logLinesBefore = (Get-Content $logPath | Measure-Object -Line).Lines }

$env:BURNMON_UICHECK = "1"
$proc = Start-Process -FilePath $exe -PassThru
try {
    # Poll the eval channel instead of a fixed sleep: it appears only once
    # the window and its bmUICheckResult binding are actually up.
    $deadline = (Get-Date).AddSeconds(20)
    $ready = $false
    while ((Get-Date) -lt $deadline) {
        try {
            $client = New-Object System.Net.Sockets.TcpClient
            $client.Connect("127.0.0.1", 9333)
            $client.Close()
            $ready = $true
            break
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $ready) {
        Write-Host "Dev eval port 9333 never came up. Either BURNMON_UICHECK is not being read, or the window failed to start; check %LOCALAPPDATA%\burnmon\burnmon-app.log."
        exit 1
    }
    # The port opens as soon as bmUICheckResult is bound, well before the
    # startup goroutine's first Collect (W1's own fix) finishes; an eval
    # sent before that can time out waiting for the UI thread. Wait for
    # app.log's own "stage Collect (total)" line instead of a fixed sleep,
    # since Collect's real duration varies a lot with machine load (seen
    # anywhere from ~2s to 60s+ on this laptop during this session). The
    # log is append-only across runs (setupLog rotates at 512KB, never
    # truncates), so this only counts a match on a line added since this
    # process started, not a stale one from an earlier run.
    $collectDeadline = (Get-Date).AddSeconds(90)
    $collectDone = $false
    while ((Get-Date) -lt $collectDeadline) {
        if (Test-Path $logPath) {
            $newLines = Get-Content $logPath | Select-Object -Skip $logLinesBefore
            if ($newLines -match "stage Collect \(total\)") {
                $collectDone = $true
                break
            }
        }
        Start-Sleep -Seconds 2
    }
    if (-not $collectDone) {
        Write-Host "First Collect never finished within 90s; checks would likely time out anyway. Check %LOCALAPPDATA%\burnmon\burnmon-app.log."
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
    Remove-Item Env:\BURNMON_UICHECK -ErrorAction SilentlyContinue
    if (-not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force
    }
}
