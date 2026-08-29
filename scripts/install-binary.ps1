# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later
#Requires -Version 5

<#
.SYNOPSIS
  Install the locally built mininaru.exe into a bin directory.
.DESCRIPTION
  Installs out\mininaru.exe (build it first with `make build`).
  This is the Windows counterpart of scripts/install-binary.sh.
.PARAMETER BinDir
  Target directory. Defaults to $env:MININARU_BINDIR, then $env:LOCALAPPDATA\mininaru\bin.
.PARAMETER Uninstall
  Remove the installed executables instead of installing them.
#>

[CmdletBinding()]
param(
	[string]$BinDir,
	[switch]$Uninstall
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $BinDir) {
	if ($env:MININARU_BINDIR) {
		$BinDir = $env:MININARU_BINDIR
	}
	else {
		$BinDir = Join-Path $env:LOCALAPPDATA 'mininaru\bin'
	}
}

if ($Uninstall) {
	foreach ($name in @('mininaru.exe', 'narush.exe')) {
		$path = Join-Path $BinDir $name
		if (Test-Path $path) {
			Remove-Item $path -Force
		}
	}
	Write-Host "removed mininaru from $BinDir"
	return
}

$outDir = if ($env:MININARU_OUT) { $env:MININARU_OUT } else { 'out' }
$source = Join-Path $outDir 'mininaru.exe'
if (-not (Test-Path $source)) {
	throw "no built binary at $source, run ``make build`` first"
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
Copy-Item $source (Join-Path $BinDir 'mininaru.exe') -Force
Write-Host "installed mininaru into $BinDir"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $BinDir) {
	[Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDir", 'User')
	Write-Host "added $BinDir to your user PATH; open a new shell for it to take effect"
}
