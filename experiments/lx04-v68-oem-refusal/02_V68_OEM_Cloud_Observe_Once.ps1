param([string]$Device = '192.168.0.192:5555')
# V68: one 25-second Frida observation with sanitized per-instruction diagnostics.
# No suppression, ASR rewrite, NLP replay, audio mute, DeepSeek handoff, or APK change.
$ErrorActionPreference = 'Stop'
$base = 'D:\Codex\XiaoSeek-apk'
$adb = 'D:\Codex\platform-tools\adb.exe'
$frida = Join-Path $base 'v53-frida-16.6.6\frida-inject.exe'
$probe = Join-Path $PSScriptRoot 'v68_cloud_refusal_trace.js'
$cleanup = Join-Path $PSScriptRoot 'device_v68_cleanup.sh'
$remote = '/data/local/tmp/xiaoseek_v53_phase_a'
$server = "$remote/frida-server"
$startHelper = "$remote/device_v53.sh"
$remoteCleanup = "$remote/device_v68_cleanup.sh"
$work = Join-Path $base ('v68-oem-refusal-trace\' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
New-Item -ItemType Directory -Path $work -Force | Out-Null
$env:TEMP=$work; $env:TMP=$work; $env:TMPDIR=$work
$report=Join-Path $work 'report.txt'
function Note([string]$s){Write-Host $s;Add-Content -LiteralPath $report -Encoding UTF8 -Value $s}
function TextCommand([string]$exe,[string[]]$argv,[int]$secs=20){
    $p=New-Object Diagnostics.Process;$pi=New-Object Diagnostics.ProcessStartInfo
    $pi.FileName=$exe;$pi.Arguments=($argv|ForEach-Object{'"'+($_.Replace('"','\"'))+'"'})-join ' '
    $pi.WorkingDirectory=$work;$pi.UseShellExecute=$false;$pi.CreateNoWindow=$true
    $pi.RedirectStandardOutput=$true;$pi.RedirectStandardError=$true;$p.StartInfo=$pi
    try{if(-not $p.Start()){throw 'START_FAILED'};$o=$p.StandardOutput.ReadToEndAsync();$e=$p.StandardError.ReadToEndAsync();if(-not $p.WaitForExit($secs*1000)){try{$p.Kill()}catch{};throw 'TIMEOUT'};[void]$o.Wait(2000);[void]$e.Wait(2000);if($p.ExitCode -ne 0){throw "COMMAND_EXIT_$($p.ExitCode): $($e.Result)"};return $o.Result.Trim()}finally{$p.Dispose()}
}
function Adb([string[]]$a,[int]$secs=20){TextCommand $adb (@('-s',$Device)+$a) $secs}
function OemPid{$ps=Adb @('shell','ps -A');$rows=@($ps -split '[\r\n]+'|Where-Object{$_ -match '\scom\.xiaomi\.micolauncher\s*$'});if($rows.Count -ne 1){throw "OEM_PROCESS_COUNT_$($rows.Count)"};$cols=@($rows[0].Trim()-split '\s+');if($cols.Count -lt 2 -or $cols[1] -notmatch '^\d+$'){throw 'OEM_PID_PARSE_FAILED'};return [string]$cols[1]}
function Tracer([string]$oemPid){$s=Adb @('shell',"su -c 'cat /proc/$oemPid/status'");$m=[regex]::Match($s,'(?m)^TracerPid:\s*(\d+)\s*$');if(-not $m.Success){throw 'TRACER_UNAVAILABLE'};return $m.Groups[1].Value}
function ReadShared([string]$path){if(-not(Test-Path -LiteralPath $path)){return ''};$f=[IO.FileStream]::new($path,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite);try{$r=[IO.StreamReader]::new($f,[Text.Encoding]::UTF8,$true);try{return $r.ReadToEnd()}finally{$r.Dispose()}}finally{$f.Dispose()}}
$client=$null;$fo=$null;$fe=$null;$copyOut=$null;$copyErr=$null
$attempted=$false;$serverStarted=$false;$preflightReady=$false;$oemBefore='';$ok=$false;$ready=$false
$stdoutPath=Join-Path $work 'frida-stdout-PRIVATE.txt';$stderrPath=Join-Path $work 'frida-stderr-PRIVATE.txt'
try{
    Note 'V68_MODE=OEM_CLOUD_REFUSAL_TRACE_ONCE'
    Note 'V68_OBSERVE_ONLY=1 INTERCEPT=OFF ASR_REWRITE=OFF NLP_REPLAY=OFF DEEPSEEK_HANDOFF=OFF GLOBAL_MUTE=OFF'
    foreach($p in @($adb,$frida,$probe,$cleanup)){if(-not(Test-Path -LiteralPath $p -PathType Leaf)){throw "LOCAL_FILE_MISSING $p"}}
    $src=[IO.File]::ReadAllText($probe,[Text.Encoding]::UTF8);if($src -notmatch "var V68_MODE = 'OBSERVE';" -or $src -notmatch 'V68_WINDOW_MS = 25000'){throw 'PROBE_NOT_OBSERVE_ONLY_ABORT'}
    if((TextCommand $frida @('--version') 5) -ne '16.6.6'){throw 'FRIDA_CLIENT_VERSION_MISMATCH'}
    if((Adb @('get-state')) -ne 'device'){throw 'ADB_NOT_READY'}
    $oemBefore=OemPid;Note "V68_OEM_BEFORE_PID=$oemBefore";if((Tracer $oemBefore) -ne '0'){throw 'OEM_ALREADY_TRACED_ABORT'}
    $ps=Adb @('shell','ps -A');if($ps -match 'frida-server|frida-gadget|frida-inject'){throw 'EXISTING_FRIDA_PROCESS_ABORT_RUN_CLEANUP_FIRST'}
    $mark=Adb @('shell',"if [ -e $remote/owned-server.pid ]; then echo PRESENT; else echo ABSENT; fi");if($mark -ne 'ABSENT'){throw 'EXISTING_OWNED_MARKER_ABORT_RUN_CLEANUP_FIRST'}
    $apkHash=Adb @('shell','sha256sum /system/app/MicoLauncher/MicoLauncher.apk') 45;if($apkHash -notmatch '^803b6d89ef20df27f563e4cbec109ef7bec1d83c9bdc67990789fc72dd5be71a\s'){throw 'REAL_APK_HASH_MISMATCH_ABORT'};Note 'V68_REAL_APK_SHA256_MATCHED=1'
    $hash=Adb @('shell',"sha256sum $server") 30;if($hash -notmatch '^07ba6529925dbcb4a3214a3293b8b31dbbfb0c74efe3c1e4f937ebb46be502ab\s'){throw 'FRIDA_SERVER_SHA256_MISMATCH'};Note 'V68_FRIDA_SERVER_SHA256_MATCHED=1'
    [void](Adb @('shell',"su -c 'test -f $startHelper && test ! -e $remote/owned-server.pid && test ! -e $remote/owned-dummy.pid'"));$preflightReady=$true
    [void](Adb @('push',$cleanup,$remoteCleanup) 30)
    $attempted=$true
    $readyText=Adb @('shell',"su -c 'sh $startHelper start-server'") 20
    if($readyText -notmatch 'V53_SERVER_READY server_pid=(\d+)'){throw "SERVER_START_FAILED $readyText"};$serverStarted=$true;Note 'V68_FRIDA_SERVER_OWNED_STARTED=1'
    $fo=[IO.File]::Open($stdoutPath,[IO.FileMode]::Create,[IO.FileAccess]::Write,[IO.FileShare]::ReadWrite);$fe=[IO.File]::Open($stderrPath,[IO.FileMode]::Create,[IO.FileAccess]::Write,[IO.FileShare]::ReadWrite)
    $pi=New-Object Diagnostics.ProcessStartInfo;$pi.FileName=$frida;$pi.Arguments='-D "'+$Device+'" -p '+$oemBefore+' -s "'+$probe+'" -i';$pi.WorkingDirectory=$work;$pi.UseShellExecute=$false;$pi.CreateNoWindow=$true;$pi.RedirectStandardInput=$true;$pi.RedirectStandardOutput=$true;$pi.RedirectStandardError=$true
    $client=New-Object Diagnostics.Process;$client.StartInfo=$pi;if(-not $client.Start()){throw 'FRIDA_CLIENT_START_FAILED'}
    $copyOut=$client.StandardOutput.BaseStream.CopyToAsync($fo);$copyErr=$client.StandardError.BaseStream.CopyToAsync($fe)
    $clock=[Diagnostics.Stopwatch]::StartNew();$lastPidCheck=-10.0
    while($clock.Elapsed.TotalSeconds -lt 40){Start-Sleep -Milliseconds 350;$fo.Flush();$fe.Flush();$data=ReadShared $stdoutPath
        if(-not $ready -and $data.Contains('V68_HOOK_ASSIGNED')){$ready=$true;Note 'V68_HOOK_ASSIGNED=1';Note 'V68_SAY_ONE_PREVIOUSLY_VERIFIED_HARMLESS_REFUSAL_QUERY_NOW=1; speak once only: 介绍一下延迟器'}
        if($data.Contains('V68_HOOK_REMOVED') -or $data.Contains('V68_INSTALL_ABORT') -or $client.HasExited){break}
        if(($clock.Elapsed.TotalSeconds-$lastPidCheck) -gt 3.0){$lastPidCheck=$clock.Elapsed.TotalSeconds;if((OemPid) -ne $oemBefore){throw 'OEM_PID_CHANGED_DURING_HOOK_ABORT'}}
    }
    if(-not $client.HasExited){try{$client.StandardInput.Close()}catch{};if(-not $client.WaitForExit(8000)){Note 'V68_FRIDA_HOST_CLIENT_TIMEOUT_LOCAL_KILL_ONLY=1';try{$client.Kill()}catch{};[void]$client.WaitForExit(2000)}}
    [void]$copyOut.Wait(3000);[void]$copyErr.Wait(3000);$fo.Flush();$fe.Flush();$data=ReadShared $stdoutPath
    foreach($line in ([regex]::Matches($data,'(?m)^V68_EVENT .*?$')|ForEach-Object{$_.Value})){Note $line}
    $m=[regex]::Match($data,'V68_HOOK_REMOVED cap=(\d+) capWithDialog=(\d+) capNoDialog=(\d+) asrFinal=(\d+) start=(\d+) toastInstruction=(\d+) speakInstruction=(\d+) finish=(\d+) nativeConflict=(\d+) toastOp=(\d+) speakOp=(\d+) earlyCandidate=(\d+) complete=(\d+) queryMatch=(\d+) refusalKnown=(\d+) speakMatchesToast=(\d+) failOpen=(\d+) mode=OBSERVE')
    if($m.Success){Note ('V68_HOOK_COUNTS cap={0} capWithDialog={1} capNoDialog={2} asrFinal={3} start={4} toastInstruction={5} speakInstruction={6} finish={7} nativeConflict={8} toastOp={9} speakOp={10} earlyCandidate={11} complete={12} queryMatch={13} refusalKnown={14} speakMatchesToast={15} failOpen={16}' -f $m.Groups[1].Value,$m.Groups[2].Value,$m.Groups[3].Value,$m.Groups[4].Value,$m.Groups[5].Value,$m.Groups[6].Value,$m.Groups[7].Value,$m.Groups[8].Value,$m.Groups[9].Value,$m.Groups[10].Value,$m.Groups[11].Value,$m.Groups[12].Value,$m.Groups[13].Value,$m.Groups[14].Value,$m.Groups[15].Value,$m.Groups[16].Value,$m.Groups[17].Value);$ok=([int]$m.Groups[1].Value -gt 0 -and [int]$m.Groups[17].Value -eq 0)}else{Note 'V68_HOOK_REMOVE_SUMMARY_MISSING=1'}
    Note "V68_FRIDA_CLIENT_EXIT=$($client.ExitCode)"
    $stderr=ReadShared $stderrPath;if($stderr.Contains('Error:') -or $stderr.Contains('Traceback')){$ok=$false;Note 'V68_FRIDA_CLIENT_STDERR_ERROR=1'}
}catch{Note "V68_TEST_ERROR=$($_.Exception.Message)";$ok=$false}
finally{
    if($client){try{if(-not $client.HasExited){try{$client.StandardInput.Close()}catch{};if(-not $client.WaitForExit(3500)){try{$client.Kill()}catch{};[void]$client.WaitForExit(2000)}}}catch{};$client.Dispose()}
    if($fo){$fo.Dispose()};if($fe){$fe.Dispose()}
    if($attempted){try{$clean=Adb @('shell',"su -c 'sh $remoteCleanup stop $oemBefore'") 25;foreach($line in ($clean -split '[\r\n]+')){if($line){Note $line}};if($clean -notmatch 'V68_CLEANUP_PASS=1'){$ok=$false}}catch{Note "V68_OWNED_SERVER_CLEANUP_ERROR=$($_.Exception.Message)";$ok=$false}}
    if($preflightReady){try{$after=OemPid;$trace=Tracer $after;Note "V68_OEM_AFTER_PID=$after V68_OEM_AFTER_TRACER_PID=$trace";if($after -ne $oemBefore -or $trace -ne '0'){$ok=$false;Note 'V68_OEM_SAFETY_POSTCHECK_FAILED=1'}}catch{Note "V68_POSTCHECK_ERROR=$($_.Exception.Message)";$ok=$false}}
    if($ok){Note 'V68_REAL_DEVICE_TRACE_OBSERVE_PASS=1 (NOT refusal-classification PASS)'}else{Note 'V68_NOT_PASSED_NO_AUTO_RETRY=1'}
    Note "REPORT=$report";Note 'Share report.txt only; keep PRIVATE Frida output on D:.'
}