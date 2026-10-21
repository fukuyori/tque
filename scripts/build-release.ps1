[CmdletBinding()]
param(
    [ValidateSet('windows','darwin','linux')][string]$OS = 'windows',
    [ValidateSet('amd64','arm64')][string]$Arch = 'amd64',
    [string]$OutputDirectory
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repo 'dist' }
$OutputDirectory = $PSCmdlet.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
$version = (Get-Content -LiteralPath (Join-Path $repo 'VERSION') -Raw).Trim()
if ($version -notmatch '^\d+\.\d+\.\d+$') { throw 'VERSION must be major.minor.patch' }
$extension = if ($OS -eq 'windows') { '.exe' } else { '' }
$output = Join-Path $OutputDirectory "tque-$OS-$Arch$extension"
$previousOS, $previousArch, $previousCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
Push-Location $repo
try {
    New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
    $env:GOOS = $OS; $env:GOARCH = $Arch; $env:CGO_ENABLED = '0'
    & go build -mod=readonly -trimpath -ldflags '-s -w' -o $output .
    if ($LASTEXITCODE -ne 0) { throw 'Release build failed' }
    Write-Output "Built tque $version ($OS/$Arch): $output"
} finally {
    $env:GOOS = $previousOS; $env:GOARCH = $previousArch; $env:CGO_ENABLED = $previousCGO
    Pop-Location
}
