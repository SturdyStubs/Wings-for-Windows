param([string]$Root,[string]$ServerID,[string]$PipeName,[switch]$Cleanup)
$ErrorActionPreference='Stop'
$report=[ordered]@{passed=$false;error=$null}
try {
 if($Cleanup){& (Join-Path $Root 'bin\wings-windows.exe') --remove-credential $ServerID | Out-Null;if($LASTEXITCODE -ne 0){throw 'Credential cleanup failed'}}
 else {
  $pipe=[IO.Pipes.NamedPipeClientStream]::new('.',$PipeName,[IO.Pipes.PipeDirection]::In,[IO.Pipes.PipeOptions]::None,[Security.Principal.TokenImpersonationLevel]::Identification)
  try{$pipe.Connect(10000);$reader=[IO.StreamReader]::new($pipe);try{$payload=$reader.ReadToEnd()|ConvertFrom-Json}finally{$reader.Dispose()}}finally{$pipe.Dispose()}
  $secure=ConvertTo-SecureString $payload.password -AsPlainText -Force;$payload.password=$null
  try{& (Join-Path $Root 'bin\Store-GameCredential.ps1') -ServerUUID $ServerID -Credential ([PSCredential]::new($payload.username,$secure))|Out-Null}finally{$secure.Dispose()}
 }
 $report.passed=$true
} catch {$report.error=$_.Exception.Message}
$file='bootstrap-report.json';if($Cleanup){$file='credential-cleanup.json'}
[IO.File]::WriteAllText((Join-Path $Root ('logs\'+$file)),($report|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
