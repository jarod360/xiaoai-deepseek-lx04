$ErrorActionPreference='Stop'
$files=@('00_V68_Cleanup_Existing_Server.ps1','02_V68_OEM_Cloud_Observe_Once.ps1','03_V68_Syntax_Check.ps1')
$failed=$false
foreach($name in $files){
  $path=Join-Path $PSScriptRoot $name
  $tokens=$null;$errors=$null
  [void][System.Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors)
  if($errors.Count -eq 0){Write-Host "V68_PS_SYNTAX_PASS $name"}else{$failed=$true;Write-Host "V68_PS_SYNTAX_FAIL $name";foreach($e in $errors){Write-Host $e.Message}}
}
if($failed){throw 'V68_PS_SYNTAX_FAILED'}
Write-Host 'V68_PS_SYNTAX_ALL_PASS=1'