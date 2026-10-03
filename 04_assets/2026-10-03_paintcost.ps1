# runs in: PowerShell, any cwd. Times the page's own paintTick against ring size (fake mode, synthetic data).
param([string]$Exe = "C:\Users\WILCOD~1\AppData\Local\Temp\claude\C--ZND-50-projects-burnmon\1560ae67-6e88-48b5-b9c0-f7478065125c\scratchpad\head-dev.exe", [string]$Key = "none")
$ErrorActionPreference = "Stop"
function Eval([string]$script) {
  $c = New-Object System.Net.Sockets.TcpClient
  try {
    $c.Connect("127.0.0.1", 9334); $s = $c.GetStream()
    $req = (@{ script = $script } | ConvertTo-Json -Compress) + "`n"
    $b = [Text.Encoding]::UTF8.GetBytes($req); $s.Write($b, 0, $b.Length)
    $c.ReceiveTimeout = 60000
    return (New-Object IO.StreamReader($s)).ReadLine()
  } finally { $c.Close() }
}
$env:BURNMON_DEV_UICHECK = "1"
$p = Start-Process -FilePath $Exe -PassThru
$deadline = (Get-Date).AddSeconds(40)
while ((Get-Date) -lt $deadline) { try { if ((Eval "1+1") -match '"ok":true') { break } } catch { }; Start-Sleep -Milliseconds 500 }
Start-Sleep -Seconds 4
if ($Key -ne "none") { Eval ("document.dispatchEvent(new KeyboardEvent('keydown',{key:'" + $Key + "',bubbles:true}))") | Out-Null; Start-Sleep -Seconds 3 }
foreach ($min in 1, 5, 15, 30, 60, 90) {
  $js = @"
(function(MIN){
  var nowMs = Date.now(), windowMs = 30 * 60 * 1000, ws = nowMs - windowMs;
  var sysMin = Math.min(MIN, 30), sys = [], groups = [], chart = [];
  for(var t = nowMs - sysMin * 60000; t <= nowMs; t += 1000){
    var s = t / 1000, iso = new Date(t).toISOString();
    sys.push({ Ts: iso, CPUPct: 35 + 25 * Math.sin(s / 90), MemUsedMB: 9000, MemTotalMB: 16000, DiskReadBps: 1e6, DiskWriteBps: 4e5, NetDownBps: 2e5, NetUpBps: 1e5, GPUPct: 12 });
  }
  var H = ['claude', 'codex', 'copilot-vscode', 'node'];
  for(var t2 = nowMs - MIN * 60000; t2 <= nowMs; t2 += 1000){
    var iso2 = new Date(t2).toISOString();
    H.forEach(function(h){ groups.push({ Ts: iso2, Harness: h, CPUPct: 4, MemMB: 400, IOBps: 100 }); });
  }
  for(var i = 0; i < 30; i++) chart.push({ at: new Date(ws + i * 60000).toISOString(), by_session: {}, cost: 0 });
  var nowIso = new Date(nowMs).toISOString();
  var snap = { now: nowMs, hist_gap_ms: 25000, sysmon_history: sys,
    process_groups_now: H.map(function(h){ return { Ts: nowIso, Harness: h, CPUPct: 4, MemMB: 400, IOBps: 100 }; }),
    process_groups_history: groups, vendor_strip: { rows: [] }, activity_heatmap: { rows: [] },
    todo: { enabled: false }, todo_tasks: { items: [] }, headline_today: 0,
    burn: { generated_at: nowIso, running_window_seconds: 1800, sessions: [], window_start: new Date(ws).toISOString(), bucket_seconds: 60, chart: chart, turns: [] } };
  window.__bdevEnterFakeMode();
  var ts = [];
  for(var k = 0; k < 15; k++){ var a = performance.now(); window.__bdevPaintFake(snap); ts.push(performance.now() - a); }
  window.__bdevExitFakeMode();
  ts.sort(function(a, b){ return a - b; });
  return {min: MIN, groupRows: groups.length, sysRows: sys.length, p50: ts[7], max: ts[14]};
})($min)
"@
  "{0}" -f (Eval ($js -replace "`r?`n", " "))
}
Stop-Process -Id $p.Id -Force
