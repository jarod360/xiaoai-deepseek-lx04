param([string]$Device = '192.168.0.192:5555')
# V68 cleanup only: stops the exact server recorded by the owned marker; never signals OEM/Agent.
$ErrorActionPreference = 'Stop'
$base = 'D:\Codex\XiaoSeek-apk'
$adb = 'D:\Codex\platform-tools\adb.exe'
$helper = Join-Path $PSScriptRoot 'device_v68_cleanup.sh'
$remote = '/data/local/tmp/xiaoseek_v53_phase_a'
$remoteHelper = "$remote/device_v68_cleanup.sh"
$work = Join-Path $base ('v68-cleanup\' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
New-Item -ItemType Directory -Path $work -Force | Out-Null
$report = Join-Path $work 'report.txt'
function Note([string]$s) { Write-Host $s; Add-Content -LiteralPath $report -Encoding UTF8 -Value $s }
function Run([string]$exe,[string[]]$argv,[int]$secs=20) {
    $p=New-Object Diagnostics.Process; $pi=New-Object Diagnostics.ProcessStartInfo
    $pi.FileName=$exe; $pi.Arguments=($argv | ForEach-Object {'"'+($_.Replace('"','\"'))+'"'}) -join ' '
    $pi.WorkingDirectory=$work; $pi.UseShellExecute=$false; $pi.CreateNoWindow=$true
    $pi.RedirectStandardOutput=$true; $pi.RedirectStandardError=$true; $p.StartInfo=$pi
    try {
        if(-not $p.Start()){throw 'START_FAILED'}
        $o=$p.StandardOutput.ReadToEndAsync(); $e=$p.StandardError.ReadToEndAsync()
        if(-not $p.WaitForExit($secs*1000)){try{$p.Kill()}catch{};throw 'TIMEOUT'}
        [void]$o.Wait(2000); [void]$e.Wait(2000)
        if($p.ExitCode -ne 0){throw "COMMAND_EXIT_$($p.ExitCode): $($e.Result)"}
        return $o.Result.Trim()
    } finally {$p.Dispose()}
}
function Adb([string[]]$a,[int]$secs=20){Run $adb (@('-s',$Device)+$a) $secs}
function OemPid {
    $ps=Adb @('shell','ps -A')
    $rows=@($ps -split '[\r\n]+' | Where-Object {$_ -match '\scom\.xiaomi\.micolauncher\s*$'})
    if($rows.Count -ne 1){throw "OEM_PROCESS_COUNT_$($rows.Count)"}
    $cols=@($rows[0].Trim() -split '\s+')
    if($cols.Count -lt 2 -or $cols[1] -notmatch '^\d+$'){throw 'OEM_PID_PARSE_FAILED'}
    return [string]$cols[1]
}
function Tracer([string]$oemPid){
    $s=Adb @('shell',"su -c 'cat /proc/$oemPid/status'")
    $m=[regex]::Match($s,'(?m)^TracerPid:\s*(\d+)\s*$')
    if(-not $m.Success){throw 'TRACER_UNAVAILABLE'}
    return $m.Groups[1].Value
}
try {
    Note 'V68_CLEANUP_MODE=EXACT_OWNED_SERVER_ONLY'
    Note 'V68_NO_FRIDA_ATTACH=1 NO_SERVER_START=1 NO_OEM_KILL=1 NO_AGENT_CHANGE=1'
    foreach($p in @($adb,$helper)){if(-not(Test-Path -LiteralPath $p -PathType Leaf)){throw "LOCAL_FILE_MISSING $p"}}
    if((Adb @('get-state')) -ne 'device'){throw 'ADB_NOT_READY'}
    $oemBefore=OemPid; $tr=Tracer $oemBefore
    Note "V68_OEM_BEFORE_PID=$oemBefore TRACER_PID=$tr"
    if($tr -ne '0'){throw 'OEM_ALREADY_TRACED_ABORT'}
    $marker=Adb @('shell',"if [ -r $remote/owned-server.pid ]; then cat $remote/owned-server.pid; else echo ABSENT; fi")
    Note "V68_OWNED_SERVER_MARKER=$marker"
    if($marker -eq 'ABSENT'){throw 'OWNED_SERVER_MARKER_ABSENT_NO_ACTION'}
    if($marker -notmatch '^\d+$'){throw 'OWNED_SERVER_MARKER_INVALID'}
    [void](Adb @('push',$helper,$remoteHelper) 30)
    $result=Adb @('shell',"su -c 'sh $remoteHelper stop $oemBefore'") 25
    foreach($line in ($result -split '[\r\n]+')){if($line){Note $line}}
    if($result -notmatch 'V68_CLEANUP_PASS=1'){throw 'DEVICE_CLEANUP_NOT_PASSED'}
    $oemAfter=OemPid; $trAfter=Tracer $oemAfter
    $psAfter=Adb @('shell','ps -A')
    $serverRows=@($psAfter -split '[\r\n]+' | Where-Object {$_ -match '\bfrida-server\b'})
    $markerAfter=Adb @('shell',"if [ -e $remote/owned-server.pid ]; then echo PRESENT; else echo ABSENT; fi")
    Note "V68_OEM_AFTER_PID=$oemAfter TRACER_PID=$trAfter"
    Note "V68_FRIDA_SERVER_PROCESS_COUNT=$($serverRows.Count) V68_MARKER_AFTER=$markerAfter"
    if($oemAfter -ne $oemBefore -or $trAfter -ne '0' -or $serverRows.Count -ne 0 -or $markerAfter -ne 'ABSENT'){throw 'V68_POSTCHECK_FAILED'}
    Note 'V68_CLEANUP_PASS=1'
} catch { Note "V68_CLEANUP_ERROR=$($_.Exception.Message)"; Note 'V68_CLEANUP_NOT_PASSED=1' }
Note "REPORT=$report"