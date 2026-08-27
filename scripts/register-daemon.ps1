# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later
#Requires -Version 5

<#
.SYNOPSIS
  Run `mininaru serve` as a per-user Scheduled Task that starts at logon.
.DESCRIPTION
  The Windows counterpart of scripts/register-daemon.sh. Registers a Scheduled
  Task named "mininaru" for the current user; there is no Windows service wrapper.
.PARAMETER Address
  Address to bind (default: 127.0.0.1).
.PARAMETER Port
  Port to bind (default: 8223).
.PARAMETER Path
  Set NARU_PATH for the task (default: inherit / unset).
.PARAMETER Disable
  Unregister the task.
#>

[CmdletBinding()]
param(
	[string]$Address = '127.0.0.1',
	[int]$Port = 8223,
	[string]$Path,
	[switch]$Disable
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$taskName = 'mininaru'

if ($Disable) {
	if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
		Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
		Write-Host "removed the $taskName scheduled task"
	}
	return
}

$binary = (Get-Command mininaru -ErrorAction SilentlyContinue).Source
if (-not $binary) {
	$candidate = Join-Path $env:LOCALAPPDATA 'mininaru\bin\mininaru.exe'
	if (Test-Path $candidate) { $binary = $candidate }
}
if (-not $binary) { throw 'mininaru not found; run scripts/install.ps1 first' }

$arguments = "serve --host $Address --port $Port"
$action = New-ScheduledTaskAction -Execute $binary -Argument $arguments
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited

if ($Path) {
	$env:NARU_PATH = $Path
	Write-Host "note: set NARU_PATH=$Path in this session; the task inherits the machine/user environment at logon"
}

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger `
	-Settings $settings -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName $taskName

Write-Host "mininaru serve will run at logon on ${Address}:${Port}"
Write-Host "  Get-ScheduledTask -TaskName $taskName"
Write-Host "  Get-Content `"$env:USERPROFILE\.mininaru\mininaru.key`"   # bearer token for the API"
