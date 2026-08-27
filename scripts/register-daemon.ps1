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
  Data directory / NARU_PATH (default: %USERPROFILE%\.mininaru). Set as a
  persistent user environment variable and passed to the task.
.PARAMETER Disable
  Unregister the task and clear the NARU_PATH user variable.
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
if (-not $Path) {
	$Path = if ($env:NARU_PATH) { $env:NARU_PATH } else { Join-Path $env:USERPROFILE '.mininaru' }
}

if ($Disable) {
	if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
		Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
		Write-Host "removed the $taskName scheduled task"
	}
	[Environment]::SetEnvironmentVariable('NARU_PATH', $null, 'User')
	Write-Host 'cleared the NARU_PATH user environment variable'
	return
}

$binary = (Get-Command mininaru -ErrorAction SilentlyContinue).Source
if (-not $binary) {
	$candidate = Join-Path $env:LOCALAPPDATA 'mininaru\bin\mininaru.exe'
	if (Test-Path $candidate) { $binary = $candidate }
}
if (-not $binary) { throw 'mininaru not found; run scripts/install.ps1 first' }

[Environment]::SetEnvironmentVariable('NARU_PATH', $Path, 'User')
$env:NARU_PATH = $Path
Write-Host "pinned NARU_PATH=$Path as a user environment variable"

$arguments = "serve --host $Address --port $Port"
$action = New-ScheduledTaskAction -Execute $binary -Argument $arguments
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger `
	-Settings $settings -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName $taskName

Write-Host "mininaru serve will run at logon on ${Address}:${Port}"
Write-Host "  Get-ScheduledTask -TaskName $taskName"
Write-Host "  Get-Content `"$Path\mininaru.key`"   # bearer token for the API"
