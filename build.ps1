# Builds claudecost-cli.exe (console CLI) and claudecost.exe (windowed app).
# Naming convention flipped 2026-08-31: claudecost.exe is now the app. Both
# binaries check for claudecost.json next to the exe first, then fall back
# to %LOCALAPPDATA%\claudecost\claudecost.json (see cmd/claudecost-app/main.go
# appDataDir), so dropping the exe and claudecost.json in one portable
# folder works for either. Run from the project root in PowerShell:
# .\build.ps1
# First run needs internet access (go mod tidy + go-winres download).
# If build.local.ps1 exists next to this script (gitignored, machine-local),
# it runs afterward for any private post-build step, such as copying the
# built app into another folder outside this repo.
$ErrorActionPreference = "Stop"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Go is not installed. Install it first: winget install GoLang.Go, then re-run."
    exit 1
}
Set-Location $PSScriptRoot
$goWinres = "go-winres"
if (-not (Get-Command go-winres -ErrorAction SilentlyContinue)) {
    $gopathBin = Join-Path (go env GOPATH) "bin\go-winres.exe"
    if (-not (Test-Path $gopathBin)) {
        Write-Host "Installing go-winres (one-time)..."
        go install github.com/tc-hib/go-winres@latest
    }
    $goWinres = $gopathBin
}
& $goWinres make --in winres\cli.json --out cmd\claudecost\rsrc
if ($LASTEXITCODE -ne 0) { Write-Host "go-winres failed for CLI; building without icon." }
& $goWinres make --in winres\app.json --out cmd\claudecost-app\rsrc
if ($LASTEXITCODE -ne 0) { Write-Host "go-winres failed for app; building without icon." }
go mod tidy
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o claudecost-cli.exe .\cmd\claudecost
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go build -trimpath -ldflags "-s -w -H windowsgui" -o claudecost.exe .\cmd\claudecost-app
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built claudecost-cli.exe (CLI) and claudecost.exe (app)"
$localHook = Join-Path $PSScriptRoot "build.local.ps1"
if (Test-Path $localHook) {
    & $localHook
}
