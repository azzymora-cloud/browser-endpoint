#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Enable Wake-on-LAN (magic packet) on Ethernet adapters.

.DESCRIPTION
  Turns on the NIC flags Windows needs so a magic packet can bring the
  ThinkCentre out of sleep or a normal shutdown (S3/S4/S5), if the BIOS
  also allows PME / Wake on LAN. Does not open router ports.
#>
$ErrorActionPreference = 'Continue'

function Write-Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

Write-Step "Enabling wake timers and NIC magic-packet wake"

powercfg /SETACVALUEINDEX SCHEME_CURRENT SUB_SLEEP RTCWAKE 1 | Out-Null
powercfg /SETDCVALUEINDEX SCHEME_CURRENT SUB_SLEEP RTCWAKE 1 | Out-Null
powercfg /change standby-timeout-ac 0
powercfg /hibernate off | Out-Null

$adapters = Get-NetAdapter -Physical -ErrorAction SilentlyContinue |
    Where-Object { $_.Status -eq 'Up' -or $_.MediaType -match '802.3|Ethernet' }

if (-not $adapters) {
    Write-Host "No Ethernet adapters found."
}

foreach ($nic in $adapters) {
    Write-Host "Adapter: $($nic.Name)  MAC=$($nic.MacAddress)"
    try {
        Enable-NetAdapterPowerManagement -Name $nic.Name -WakeOnMagicPacket Enabled -ErrorAction Stop
        Write-Host "  WakeOnMagicPacket enabled"
    } catch {
        Write-Host "  WakeOnMagicPacket: $($_.Exception.Message)"
    }

    $pnp = Get-PnpDeviceProperty -InstanceId $nic.PnPDeviceID -KeyName 'DEVPKEY_Device_DeviceDesc' -ErrorAction SilentlyContinue
    $devName = $nic.InterfaceDescription
    if ($devName) {
        powercfg /deviceenablewake "$devName" 2>$null | Out-Null
    }

    $power = Get-CimInstance -Namespace root\wmi -ClassName MSPower_DeviceWakeEnable -ErrorAction SilentlyContinue |
        Where-Object { $_.InstanceName -match [regex]::Escape($nic.PnPDeviceID.Replace('\', '_')) -or $_.InstanceName -match [regex]::Escape($nic.Name) }
    foreach ($row in $power) {
        if (-not $row.Enable) {
            $row.Enable = $true
            $row | Set-CimInstance
        }
    }
}

# Disable "allow the computer to turn off this device" where we can see the setting.
Get-NetAdapterPowerManagement -ErrorAction SilentlyContinue | ForEach-Object {
    try {
        Set-NetAdapterPowerManagement -Name $_.Name -AllowComputerToTurnOffDevice Disabled -ErrorAction SilentlyContinue
    } catch {}
}

Write-Step "BIOS / firmware (you must confirm once on the ThinkCentre)"
Write-Host "Reboot into Setup (usually F1 on ThinkCentre) and enable:"
Write-Host "  - Power / Wake on LAN"
Write-Host "  - Power / After Power Loss = Power On  (comes back after an outage)"
Write-Host "  - Disable Deep S4/S5 or 'Wake on LAN from S5' if that option exists"
Write-Host ""
Write-Host "To wake a machine that is already off, send a magic packet from a phone"
Write-Host "on the home LAN, or from another PC:"
Write-Host "  thinkcentre-agent.exe wake --mac <MAC-above>"
Write-Host "The agent running on this PC cannot wake itself if it is powered off."
