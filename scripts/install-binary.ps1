# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later
#Requires -Version 5

<#
.SYNOPSIS
  Install the locally built mininaru.exe (and narush.exe) into a bin directory.
.DESCRIPTION
  Installs out\mininaru.exe and out\narush.exe (build them first with `make build`).
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

$names = @('mininaru.exe', 'narush.exe')

if ($Uninstall) {
	foreach ($name in $names) {
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
foreach ($name in $names) {
	$from = Join-Path $outDir $name
	if (-not (Test-Path $from)) { $from = $source }
	Copy-Item $from (Join-Path $BinDir $name) -Force
}
Write-Host "installed mininaru and narush into $BinDir"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $BinDir) {
	[Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDir", 'User')
	Write-Host "added $BinDir to your user PATH; open a new shell for it to take effect"
}
