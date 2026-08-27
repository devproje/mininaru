# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later
#Requires -Version 5

<#
.SYNOPSIS
  Install mininaru from a GitHub release on Windows.
.DESCRIPTION
  When mininaru is already on PATH this hands off to `mininaru update`. Otherwise
  the latest release .zip is downloaded, checked against SHA256SUMS, and installed.
.PARAMETER Tag
  Release tag to install. Defaults to the newest release (prereleases included).
.PARAMETER BinDir
  Target directory. Defaults to $env:MININARU_BINDIR, then $env:LOCALAPPDATA\mininaru\bin.
.PARAMETER Uninstall
  Remove the installed executables instead of installing them.
#>

[CmdletBinding()]
param(
	[string]$Tag,
	[string]$BinDir,
	[switch]$Uninstall
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repo = 'devproje/mininaru'
$apiBase = if ($env:MININARU_API_BASE) { $env:MININARU_API_BASE } else { 'https://api.github.com' }

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
		if (Test-Path $path) { Remove-Item $path -Force }
	}
	Write-Host "removed mininaru from $BinDir"
	return
}

if (-not $Tag -and (Get-Command mininaru -ErrorAction SilentlyContinue)) {
	Write-Host "mininaru is already installed, delegating to ``mininaru update``"
	mininaru update
	return
}

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
	'AMD64' { 'amd64' }
	'ARM64' { 'arm64' }
	default { throw "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
}

$headers = @{ 'Accept' = 'application/vnd.github+json' }
if ($env:GITHUB_TOKEN) { $headers['Authorization'] = "Bearer $($env:GITHUB_TOKEN)" }

$requestedTag = $Tag
if (-not $Tag) {
	Write-Host 'resolving the latest release'
	$releases = Invoke-RestMethod -Headers $headers -Uri "$apiBase/repos/$repo/releases"
	if (-not $releases) { throw 'repository has no releases' }
	$Tag = $releases[0].tag_name
}

if ($Tag -match '^v?0\.') {
	if (-not $requestedTag) {
		throw @"
the newest release is $Tag, from the pre-1.0 architecture that this rewrite
is not compatible with. Versioning restarts at v1.0.0-alpha.1 and no such
release exists yet, so there is nothing to install from a release right now --
build from source (make build) instead. Pass -Tag explicitly only if you
specifically want an old 0.x build.
"@
	}
	Write-Warning "installing $Tag, a pre-1.0 build incompatible with the current architecture"
}

$asset = "mininaru_${Tag}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$Tag"
$work = Join-Path ([System.IO.Path]::GetTempPath()) "mininaru-$([System.Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $work | Out-Null

try {
	Write-Host "downloading $asset"
	Invoke-WebRequest -Headers $headers -Uri "$base/$asset" -OutFile (Join-Path $work $asset)
	Invoke-WebRequest -Headers $headers -Uri "$base/SHA256SUMS" -OutFile (Join-Path $work 'SHA256SUMS')

	$want = $null
	foreach ($line in Get-Content (Join-Path $work 'SHA256SUMS')) {
		$fields = $line -split '\s+'
		if ($fields.Count -eq 2 -and $fields[1] -eq $asset) { $want = $fields[0].ToLower() }
	}
	if (-not $want) { throw "SHA256SUMS has no entry for $asset" }

	$got = (Get-FileHash -Algorithm SHA256 (Join-Path $work $asset)).Hash.ToLower()
	if ($got -ne $want) { throw "checksum mismatch for ${asset}: expected $want, got $got" }
	Write-Host 'checksum verified'

	Expand-Archive -Path (Join-Path $work $asset) -DestinationPath $work -Force
	$extracted = Join-Path $work "mininaru_windows_${arch}"

	New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
	foreach ($name in $names) {
		Copy-Item (Join-Path $extracted $name) (Join-Path $BinDir $name) -Force
	}
	Write-Host "installed mininaru $Tag into $BinDir"
	& (Join-Path $BinDir 'mininaru.exe') --version
}
finally {
	Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $BinDir) {
	[Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDir", 'User')
	Write-Host "added $BinDir to your user PATH; open a new shell for it to take effect"
}
