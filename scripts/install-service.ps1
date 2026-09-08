#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $root 'thinkcentre-agent.exe'
if (-not (Test-Path $exe)) {
    $fromRelease = Join-Path $root 'releases\thinkcentre-agent.exe'
    if (Test-Path $fromRelease) {
        Copy-Item $fromRelease $exe -Force
    }
}
if (-not (Test-Path $exe)) {
    throw "thinkcentre-agent.exe not found. Clone the repo (do not download files from the website) so releases/thinkcentre-agent.exe is on disk, then re-run this script."
}
& $exe install
if ($LASTEXITCODE -ne 0) { throw "agent install failed" }
& $exe enable-wol

$docker = Get-Command docker -ErrorAction SilentlyContinue
if (-not $docker) {
    $guess = Join-Path $env:ProgramFiles 'Docker\Docker\resources\bin\docker.exe'
    if (Test-Path $guess) { $docker = Get-Item $guess }
}
if ($docker) {
    $dockerPath = if ($docker.Source) { $docker.Source } else { $docker.FullName }
    $action = New-ScheduledTaskAction -Execute $dockerPath -Argument 'compose -f docker-compose.yml up -d' -WorkingDirectory $root
    $trigger = New-ScheduledTaskTrigger -AtLogOn
    Register-ScheduledTask -TaskName 'ThinkCentre Endpoint Stack' -Action $action -Trigger $trigger -RunLevel Highest -Force | Out-Null
    Write-Host "Registered logon task to start Guacamole (Docker Desktop is per-user)."
} else {
    Write-Host "Docker not found; skip logon task. Start the stack from the agent after Docker Desktop is running."
}

Write-Host "Open http://127.0.0.1:18765 and use the token in %ProgramData%\ThinkCentreEndpoint\config.json"
