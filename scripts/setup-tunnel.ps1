#Requires -Version 5.1
<#
.SYNOPSIS
  Save a named Cloudflare Tunnel token and (optionally) start cloudflared.

.DESCRIPTION
  Restricted networks that block VPNs and Parsec still allow ordinary HTTPS.
  cloudflared on the ThinkCentre dials out to Cloudflare on 443. Your browser
  opens a normal https:// hostname - no VPN client, no UPnP, no inbound ports.

  Create the named tunnel in the Cloudflare dashboard (free plan), then paste
  the token here. Optional Cloudflare Access (email one-time PIN) is configured
  in Zero Trust; this script only stores the tunnel token.
#>
[CmdletBinding()]
param(
    [string]$Token,
    [string]$Hostname,
    [string]$ManageHostname,
    [switch]$StartTunnel,
    [switch]$QuickTest
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $repoRoot '.env'
$examplePath = Join-Path $repoRoot '.env.example'

function Set-DotEnvValue {
    param(
        [string]$Path,
        [string]$Name,
        [string]$Value
    )
    $escaped = $Value -replace '\\', '\\'
    if (-not (Test-Path $Path)) {
        if (Test-Path $examplePath) {
            Copy-Item $examplePath $Path
        } else {
            New-Item -ItemType File -Path $Path | Out-Null
        }
    }
    $lines = Get-Content -Path $Path
    $found = $false
    $updated = foreach ($line in $lines) {
        if ($line -match "^\s*$([regex]::Escape($Name))\s*=") {
            $found = $true
            "$Name=$escaped"
        } else {
            $line
        }
    }
    if (-not $found) {
        $updated += "$Name=$escaped"
    }
    Set-Content -Path $Path -Value $updated -Encoding utf8
}

Write-Host "Cloudflare Tunnel setup (outbound HTTPS 443 only)"
Write-Host ""

if ($QuickTest) {
    Write-Host "Quick test (URL changes every restart - not for daily use):"
    Write-Host "  docker compose up -d"
    Write-Host "  docker run --rm --network container:thinkcentre-postgres-1 cloudflare/cloudflared:latest tunnel --no-autoupdate --url http://127.0.0.1:8080"
    Write-Host "Use the printed https://*.trycloudflare.com link once, then create a named tunnel."
    return
}

Write-Host "In a browser (on this PC or your phone):"
Write-Host "  1. Add your domain to Cloudflare (free plan) if it is not already there."
Write-Host "  2. Open https://one.dash.cloudflare.com/ -> Networks -> Tunnels -> Create tunnel."
Write-Host "  3. Choose Cloudflared, name it thinkcentre, and copy the tunnel token."
  Write-Host "  4. Add a public hostname, e.g. pc.yourdomain.com -> http://127.0.0.1:8080"
Write-Host "  5. Add a second hostname for the agent, e.g. manage.yourdomain.com -> http://host.docker.internal:18765"
Write-Host "     (Agent must be installed and listening on 127.0.0.1:18765.)"
Write-Host "     (If the connector is not up yet, you can add the hostnames after this script.)"
Write-Host "  6. Optional hardening: Zero Trust → Access → Applications → Self-hosted"
Write-Host "     for both hostnames, policy = your email + One-time PIN."
Write-Host ""

if (-not $Token) {
    $secure = Read-Host "Paste the Cloudflare Tunnel token" -AsSecureString
    $Token = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
        [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    )
}
if ([string]::IsNullOrWhiteSpace($Token)) {
    throw "A tunnel token is required. Re-run after creating the tunnel in Zero Trust."
}

if (-not $Hostname) {
    $Hostname = Read-Host "Desktop hostname (e.g. pc.example.com, or blank to skip)"
}
if (-not $ManageHostname) {
    $ManageHostname = Read-Host "Manage hostname (e.g. manage.example.com, or blank to skip)"
}

Set-DotEnvValue -Path $envPath -Name 'CLOUDFLARE_TUNNEL_TOKEN' -Value $Token.Trim()
if ($Hostname) {
    Set-DotEnvValue -Path $envPath -Name 'CLOUDFLARE_HOSTNAME' -Value $Hostname.Trim()
}
if ($ManageHostname) {
    Set-DotEnvValue -Path $envPath -Name 'CLOUDFLARE_MANAGE_HOSTNAME' -Value $ManageHostname.Trim()
}

Write-Host ""
Write-Host "Wrote CLOUDFLARE_TUNNEL_TOKEN to .env"

if ($StartTunnel) {
    Push-Location $repoRoot
    try {
        docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d
    } finally {
        Pop-Location
    }
    if ($Hostname) {
        Write-Host "After the connector shows HEALTHY in Zero Trust, open https://$Hostname"
    }
} else {
    Write-Host "Start the tunnel when Guacamole already works on http://127.0.0.1:8080 :"
    Write-Host "  docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d"
    if ($Hostname) {
        Write-Host "Then open https://$Hostname from any browser - including networks that block VPNs and Parsec."
    }
}
