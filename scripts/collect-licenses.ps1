[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$OutputDirectory,
    [Parameter(Mandatory)][string]$BinaryPath,
    [Parameter(Mandatory)][ValidateSet('windows','darwin','linux')][string]$OS,
    [Parameter(Mandatory)][ValidateSet('amd64','arm64')][string]$Arch
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$OutputDirectory = $PSCmdlet.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
$BinaryPath = $PSCmdlet.GetUnresolvedProviderPathFromPSPath($BinaryPath)
$previousOS, $previousArch, $previousCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
Push-Location $repo
try {
    $hostOS = & go env GOHOSTOS
    if ($LASTEXITCODE -ne 0) { throw 'Host OS lookup failed' }
    $hostArch = & go env GOHOSTARCH
    if ($LASTEXITCODE -ne 0) { throw 'Host architecture lookup failed' }
    $env:GOOS = $hostOS; $env:GOARCH = $hostArch; $env:CGO_ENABLED = '0'
    & go run -mod=readonly ./scripts/licenses -binary $BinaryPath -out $OutputDirectory -os $OS -arch $Arch
    if ($LASTEXITCODE -ne 0) { throw 'License collection or binary verification failed' }
} finally {
    $env:GOOS = $previousOS; $env:GOARCH = $previousArch; $env:CGO_ENABLED = $previousCGO
    Pop-Location
}
