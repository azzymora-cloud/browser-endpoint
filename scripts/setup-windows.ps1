#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Prepare a Windows ThinkCentre to be remoted through Guacamole.

.DESCRIPTION
  Detects Home vs Pro. Enables inbound RDP (Pro/Enterprise/Education) or
  installs TightVNC on 5900 (Home). Tightens the Windows firewall so 3389/5900
  are not open to the internet. Turns off sleep/hibernate and unused UPnP.

  Docker Desktop talks to the host as host.docker.internal, which is not
  loopback, so listeners are not bound to 127.0.0.1 only. The firewall allows
  RFC1918 + loopback and nothing else.
#>
[CmdletBinding()]
param(
    [ValidateSet('Auto', 'RDP', 'VNC')]
    [string]$RemoteProtocol = 'Auto',

    [string]$VncPassword,

    [switch]$SkipPower,
    [switch]$SkipUpnp,
    [switch]$SkipDockerCheck
)

$ErrorActionPreference = 'Stop'

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Get-WindowsEditionId {
    return (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion').EditionID
}

function Test-RdpCapableEdition {
    param([string]$EditionId)
    $rdpEditions = @(
        'Professional', 'ProfessionalN', 'ProfessionalWorkstation', 'ProfessionalWorkstationN',
        'Enterprise', 'EnterpriseN', 'EnterpriseS', 'EnterpriseSN',
        'Education', 'EducationN',
        'ServerStandard', 'ServerDatacenter', 'ServerSolution'
    )
    return $rdpEditions -contains $EditionId
}

function Disable-WideFirewallRules {
    param(
        [string[]]$DisplayNameMatch,
        [string]$DisplayGroup
    )
    if ($DisplayGroup) {
        Get-NetFirewallRule -DisplayGroup $DisplayGroup -ErrorAction SilentlyContinue |
            Set-NetFirewallRule -Enabled False -ErrorAction SilentlyContinue
    }
    foreach ($pattern in $DisplayNameMatch) {
        Get-NetFirewallRule -ErrorAction SilentlyContinue |
            Where-Object { $_.DisplayName -like $pattern } |
            Set-NetFirewallRule -Enabled False -ErrorAction SilentlyContinue
    }
}

function New-PrivateDesktopFirewallRule {
    param(
        [string]$Name,
        [string]$DisplayName,
        [int]$LocalPort
    )
    $existing = Get-NetFirewallRule -Name $Name -ErrorAction SilentlyContinue
    if ($existing) {
        Remove-NetFirewallRule -Name $Name
    }
    New-NetFirewallRule -Name $Name `
        -DisplayName $DisplayName `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort $LocalPort `
        -RemoteAddress @('127.0.0.1', '::1', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16') `
        -Profile Any |
        Out-Null
}

function Enable-ThinkCentreRdp {
    Write-Step "Enabling Remote Desktop (NLA on)"
    Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -Value 0
    Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -Name UserAuthentication -Value 1
    Disable-WideFirewallRules -DisplayGroup 'Remote Desktop'
    New-PrivateDesktopFirewallRule -Name 'ThinkCentre-RDP-RFC1918' -DisplayName 'ThinkCentre RDP (RFC1918 + loopback)' -LocalPort 3389
    Write-Host "RDP is on. Firewall allows 3389 only from loopback and private LAN/Docker ranges."
}

function Install-TightVncServer {
    param([string]$Password)

    if (-not $Password) {
        $secure = Read-Host "TightVNC password (8 characters max)" -AsSecureString
        $Password = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
            [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
        )
    }
    if ([string]::IsNullOrWhiteSpace($Password)) {
        throw "A TightVNC password is required on Windows Home."
    }
    if ($Password.Length -gt 8) {
        throw "TightVNC passwords are at most 8 characters."
    }

    Write-Step "Installing TightVNC server (Windows Home has no RDP host)"
    $version = '2.8.85'
    $msiName = "tightvnc-$version-gpl-setup-64bit.msi"
    $url = "https://www.tightvnc.com/download/$version/$msiName"
    $msi = Join-Path $env:TEMP $msiName

    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $msi -UseBasicParsing

    $log = Join-Path $env:TEMP 'tightvnc-install.log'
    $args = @(
        '/i', $msi, '/qn', '/norestart',
        "/l*v", $log,
        'ADDLOCAL=Server',
        'SERVER_REGISTER_AS_SERVICE=1',
        'SERVER_ADD_FIREWALL_EXCEPTION=0',
        'SET_USEVNCAUTHENTICATION=1', 'VALUE_OF_USEVNCAUTHENTICATION=1',
        'SET_PASSWORD=1', "VALUE_OF_PASSWORD=$Password",
        'SET_USECONTROLAUTHENTICATION=1', 'VALUE_OF_USECONTROLAUTHENTICATION=1',
        'SET_CONTROLPASSWORD=1', "VALUE_OF_CONTROLPASSWORD=$Password",
        'SET_ACCEPTHTTPCONNECTIONS=1', 'VALUE_OF_ACCEPTHTTPCONNECTIONS=0'
    )
    $proc = Start-Process msiexec.exe -ArgumentList $args -Wait -PassThru
    if ($proc.ExitCode -ne 0 -and $proc.ExitCode -ne 3010) {
        throw "TightVNC installer failed with exit code $($proc.ExitCode). See $log"
    }

    Disable-WideFirewallRules -DisplayNameMatch @('TightVNC*')
    New-PrivateDesktopFirewallRule -Name 'ThinkCentre-VNC-RFC1918' -DisplayName 'ThinkCentre VNC (RFC1918 + loopback)' -LocalPort 5900

    $service = Get-Service -Name 'tvnserver' -ErrorAction SilentlyContinue
    if ($service) {
        Set-Service -Name 'tvnserver' -StartupType Automatic
        if ($service.Status -ne 'Running') {
            Start-Service -Name 'tvnserver'
        }
    }
    Write-Host "TightVNC is listening on 5900. Use connection ThinkCentre (VNC) in Guacamole."
}

function Set-AlwaysOnPower {
    Write-Step "Disabling sleep and hibernate (the endpoint must stay reachable)"
    powercfg /hibernate off | Out-Null
    powercfg /change standby-timeout-ac 0
    powercfg /change hibernate-timeout-ac 0
    powercfg /change disk-timeout-ac 0
}

function Disable-UpnpServices {
    Write-Step "Disabling UPnP (this stack never needs inbound port mapping)"
    foreach ($name in @('SSDPSRV', 'upnphost')) {
        $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
        if (-not $svc) { continue }
        Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
        Set-Service -Name $name -StartupType Disabled -ErrorAction SilentlyContinue
    }
}

function Test-DockerDesktop {
    Write-Step "Checking Docker Desktop"
    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if ($docker) {
        Write-Host "Docker CLI found at $($docker.Source)"
        return
    }
    Write-Host "Docker Desktop is not on PATH yet."
    Write-Host "Install it (WSL2 backend), then reboot if asked:"
    Write-Host "  https://docs.docker.com/desktop/setup/install/windows-install/"
}

$editionId = Get-WindowsEditionId
$caption = (Get-CimInstance Win32_OperatingSystem).Caption
$rdpCapable = Test-RdpCapableEdition -EditionId $editionId

Write-Host "ThinkCentre Windows setup"
Write-Host "  OS:      $caption"
Write-Host "  Edition: $editionId"

$protocol = $RemoteProtocol
if ($protocol -eq 'Auto') {
    $protocol = if ($rdpCapable) { 'RDP' } else { 'VNC' }
}

if ($protocol -eq 'RDP' -and -not $rdpCapable) {
    throw "This edition ($editionId) cannot host Remote Desktop. Re-run with -RemoteProtocol VNC."
}

if ($protocol -eq 'RDP') {
    Enable-ThinkCentreRdp
} else {
    Install-TightVncServer -Password $VncPassword
}

if (-not $SkipPower) {
    Set-AlwaysOnPower
}
if (-not $SkipUpnp) {
    Disable-UpnpServices
}
if (-not $SkipDockerCheck) {
    Test-DockerDesktop
}

Write-Step "Next"
Write-Host "1. Copy this repo onto the ThinkCentre if it is not already here."
Write-Host "2. Copy .env.example to .env and set POSTGRES_PASSWORD."
Write-Host "3. docker compose up -d"
Write-Host "4. Open http://127.0.0.1:8080  (guacadmin / guacadmin — change it immediately)"
Write-Host "5. Click ThinkCentre ($protocol), then run scripts/setup-tunnel.ps1"
Write-Host ""
Write-Host "Do not forward 3389 or 5900 on the router. Cloudflare Tunnel is the only way in."
