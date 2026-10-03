# runs in: PowerShell, any cwd. Measures burnmon-dev CPU per process group with the Station open (or closed).
# usage: .\measure.ps1 -Exe <path> -Label <name> -Minutes 30 -Key p|none [-ThemeKey t]
param(
  [string]$Exe = "C:\ZND\50_projects\burnmon\burnmon-dev.exe",
  [string]$Label = "run",
  [int]$Minutes = 30,
  [string]$Key = "p",
  [string]$ThemeKey = "",
  [string]$Pre = "",
  [string]$Place = ""
)
$ErrorActionPreference = "Stop"
$out = Join-Path (Split-Path $PSCommandPath) ("$Label.csv")
$ncpu = [Environment]::ProcessorCount

function Eval([string]$script) {
  $c = New-Object System.Net.Sockets.TcpClient
  try {
    $c.Connect("127.0.0.1", 9334)
    $s = $c.GetStream()
    $req = (@{ script = $script } | ConvertTo-Json -Compress) + "`n"
    $b = [Text.Encoding]::UTF8.GetBytes($req)
    $s.Write($b, 0, $b.Length)
    $sr = New-Object IO.StreamReader($s)
    $c.ReceiveTimeout = 15000
    return $sr.ReadLine()
  } finally { $c.Close() }
}

$env:BURNMON_DEV_UICHECK = "1"
$p = Start-Process -FilePath $Exe -PassThru
$deadline = (Get-Date).AddSeconds(40)
while ((Get-Date) -lt $deadline) {
  try { $r = Eval "1+1"; if ($r -match '"ok":true') { break } } catch { }
  Start-Sleep -Milliseconds 500
}
Start-Sleep -Seconds 5

if ($Place -ne "") {
  Add-Type -AssemblyName System.Windows.Forms
  Add-Type @"
using System; using System.Runtime.InteropServices;
public class W32 { [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
 [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h, IntPtr a, int x, int y, int w, int hh, uint f);
 [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int c); }
"@
  [void][W32]::SetProcessDPIAware()
  $scr = [System.Windows.Forms.Screen]::AllScreens | Where-Object { $_.DeviceName -like "*$Place" } | Select-Object -First 1
  $p.Refresh(); $hw = $p.MainWindowHandle
  [void][W32]::ShowWindow($hw, 9)
  $b = $scr.Bounds
  [void][W32]::SetWindowPos($hw, [IntPtr]::Zero, $b.X + 100, $b.Y + 100, [int]($b.Width * 0.8), [int]($b.Height * 0.8), 0x0040)
  Write-Host ("placed on {0} bounds {1}" -f $scr.DeviceName, $b)
  Start-Sleep -Seconds 3
}

# page-side counters, installed once
Eval "(function(){ if(window.__m) return 1; window.__m={raf:0,st:0,tk:0}; document.addEventListener('keydown',function(e){if(e.isTrusted)window.__m.tk++;},true); var r=window.requestAnimationFrame; window.requestAnimationFrame=function(f){window.__m.raf++; return r.call(window,f);}; var s=window.setTimeout; window.setTimeout=function(f,t){window.__m.st++; return s.apply(window,arguments);}; return 1;})()" | Out-Null
if ($Key -ne "none") {
  Eval ("document.dispatchEvent(new KeyboardEvent('keydown',{key:'" + $Key + "',bubbles:true}))") | Out-Null
}
if ($ThemeKey -ne "") {
  Start-Sleep -Seconds 2
  Eval ("document.dispatchEvent(new KeyboardEvent('keydown',{key:'" + $ThemeKey + "',bubbles:true}))") | Out-Null
}

if ($Pre -ne "") { Eval $Pre | Out-Null }
$info = Eval "(function(){var c=document.querySelector('.bms-canvas'),t=document.querySelector('.bms-title');return {dpr:window.devicePixelRatio,iw:innerWidth,ih:innerHeight,cw:c?c.width:-1,ch:c?c.height:-1,title:t?t.textContent:null}})()"
Write-Host "info $info"
"t_s,go_cpu_s,wv_renderer_s,wv_gpu_s,wv_browser_s,wv_other_s,go_ws_mb,go_threads,go_handles,wv_renderer_ws_mb,js_heap_mb,dom_nodes,raf_total,settimeout_total,trusted_keys" | Set-Content $out -Encoding ascii
$t0 = Get-Date
$end = $t0.AddMinutes($Minutes)
while ((Get-Date) -lt $end) {
  $procs = Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId, Name, CommandLine
  $byParent = @{}
  foreach ($q in $procs) { if (-not $byParent.ContainsKey([int]$q.ParentProcessId)) { $byParent[[int]$q.ParentProcessId] = @() }; $byParent[[int]$q.ParentProcessId] += $q }
  $desc = New-Object System.Collections.ArrayList
  $stack = New-Object System.Collections.Stack; $stack.Push([int]$p.Id)
  while ($stack.Count) { $id = $stack.Pop(); if ($byParent.ContainsKey($id)) { foreach ($ch in $byParent[$id]) { [void]$desc.Add($ch); $stack.Push([int]$ch.ProcessId) } } }
  $r = 0.0; $g = 0.0; $br = 0.0; $o = 0.0; $rws = 0.0
  foreach ($d in $desc) {
    try { $pp = [Diagnostics.Process]::GetProcessById([int]$d.ProcessId); $cpu = $pp.TotalProcessorTime.TotalSeconds; $ws = $pp.WorkingSet64 / 1MB } catch { continue }
    $cl = [string]$d.CommandLine
    if ($cl -match '--type=renderer') { $r += $cpu; $rws += $ws }
    elseif ($cl -match '--type=gpu-process') { $g += $cpu }
    elseif ($cl -notmatch '--type=') { $br += $cpu }
    else { $o += $cpu }
  }
  $gp = [Diagnostics.Process]::GetProcessById($p.Id)
  $page = Eval "(function(){var m=performance.memory;return {h:m?m.usedJSHeapSize/1048576:-1,n:document.getElementsByTagName('*').length,raf:window.__m?window.__m.raf:-1,st:window.__m?window.__m.st:-1,tk:window.__m?window.__m.tk:-1}})()"
  $h = -1; $n = -1; $raf = -1; $st = -1; $tk = -1
  if ($page -match '"h":([\-0-9\.eE]+),"n":(\d+),"raf":(\-?\d+),"st":(\-?\d+),"tk":(\-?\d+)') { $tk = $Matches[5]; $h = $Matches[1]; $n = $Matches[2]; $raf = $Matches[3]; $st = $Matches[4] }
  $line = "{0:F1},{1:F2},{2:F2},{3:F2},{4:F2},{5:F2},{6:F1},{7},{8},{9:F1},{10},{11},{12},{13},{14}" -f ((Get-Date) - $t0).TotalSeconds, $gp.TotalProcessorTime.TotalSeconds, $r, $g, $br, $o, ($gp.WorkingSet64 / 1MB), $gp.Threads.Count, $gp.HandleCount, $rws, $h, $n, $raf, $st, $tk
  Add-Content $out $line -Encoding ascii
  Start-Sleep -Seconds 10
}
Stop-Process -Id $p.Id -Force
Get-Process msedgewebview2 -ErrorAction SilentlyContinue | Out-Null
"done $out ncpu=$ncpu"
