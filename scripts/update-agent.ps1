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
    throw "thinkcentre-agent.exe not found. Build the agent, then re-run this script as Administrator."
}

$destDir = Join-Path $env:ProgramFiles 'ThinkCentre Endpoint'
$dest = Join-Path $destDir 'thinkcentre-agent.exe'
if (-not (Test-Path $destDir)) {
    throw "ThinkCentre Endpoint is not installed at $destDir"
}

$svc = Get-Service ThinkCentreEndpoint -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -ne 'Stopped') {
    Stop-Service ThinkCentreEndpoint -Force
    Start-Sleep -Seconds 2
}
Copy-Item $exe $dest -Force
if ($svc) {
    Start-Service ThinkCentreEndpoint
} else {
    & $dest install
    if ($LASTEXITCODE -ne 0) { throw "agent install failed" }
}

Write-Host "Updated ThinkCentre Endpoint Agent at $dest"
Write-Host "Connection log: $env:ProgramData\ThinkCentreEndpoint\logs\connections.jsonl"
Write-Host "Open http://127.0.0.1:18765"
