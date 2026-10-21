[CmdletBinding()]
param(
    [ValidateSet('amd64','arm64')][string]$Arch = 'amd64',
    [switch]$Sign,
    [string]$TimestampUrl,
    [string]$SignToolPath,
    [switch]$MachineCertificateStore,
    [string]$ISCCPath = 'ISCC.exe'
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
if (-not (Test-Path -LiteralPath (Join-Path $repo 'LICENSE'))) { throw 'Finalize the copyright holder and LICENSE before packaging.' }
if ((Get-Content -LiteralPath (Join-Path $repo 'LICENSE') -Raw).Contains('{{COPYRIGHT_HOLDER}}')) { throw 'LICENSE still contains an unresolved copyright holder.' }
if ($Sign) {
    if ($env:CODESIGN_CERT -notmatch '^[0-9a-fA-F]{40}$') { throw 'Confirm CODESIGN_CERT format; this script supports certificate-store SHA1 thumbprints.' }
    if ($TimestampUrl -notmatch '^https?://') { throw '-TimestampUrl is required with -Sign' }
    if (-not $SignToolPath) {
        $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
        if ($command) { $SignToolPath = $command.Source }
    }
    if (-not $SignToolPath -or -not (Test-Path -LiteralPath $SignToolPath)) { throw 'Supply -SignToolPath pointing to the Windows SDK signtool.exe' }
}
& (Join-Path $PSScriptRoot 'build-release.ps1') -OS windows -Arch $Arch
$stage = Join-Path $repo ('dist/installer-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $stage | Out-Null
Copy-Item -LiteralPath (Join-Path $repo "dist/tque-windows-$Arch.exe") -Destination (Join-Path $stage 'tque.exe')
foreach ($file in @('LICENSE','README.md','README.ja.md','CHANGELOG.md')) { Copy-Item -LiteralPath (Join-Path $repo $file) -Destination $stage }
Copy-Item -LiteralPath (Join-Path $repo 'docs') -Destination (Join-Path $stage 'docs') -Recurse
& (Join-Path $PSScriptRoot 'collect-licenses.ps1') -OutputDirectory (Join-Path $stage 'third-party-licenses')
$version = (Get-Content -LiteralPath (Join-Path $repo 'VERSION') -Raw).Trim()
$output = Join-Path $repo 'dist'
$arguments = @('/Qp',"/DAppVersion=$version","/DTargetArch=$Arch","/DStageDir=$stage","/DOutputPath=$output")
$oldTool,$oldURL,$oldStore = $env:TQUE_SIGNTOOL_PATH,$env:TQUE_SIGN_TIMESTAMP_URL,$env:TQUE_SIGN_MACHINE_STORE
try {
    if ($Sign) {
        $env:TQUE_SIGNTOOL_PATH = (Resolve-Path -LiteralPath $SignToolPath).Path
        $env:TQUE_SIGN_TIMESTAMP_URL = $TimestampUrl
        $env:TQUE_SIGN_MACHINE_STORE = if ($MachineCertificateStore) { '1' } else { '0' }
        $helper = Join-Path $PSScriptRoot 'sign-file.ps1'
        & $helper -Path (Join-Path $stage 'tque.exe')
        if ($LASTEXITCODE -ne 0) { throw 'Executable signing failed' }
        $shellPath = (Get-Command powershell.exe).Source
        # Inno's quoted filename substitution; the certificate value is inherited, not interpolated.
        $signCommand = '$q' + $shellPath + '$q -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $q' + $helper + '$q -Path $f'
        $arguments += '/DEnableSigning=1'
        $arguments += '/Stque_sign=' + $signCommand
    }
    $arguments += (Join-Path $repo 'installer/tque.iss')
    & $ISCCPath @arguments
    if ($LASTEXITCODE -ne 0) { throw 'Installer compilation/signing failed' }
    if ($Sign) {
        & $SignToolPath verify /q /pa (Join-Path $output "tque-$version-windows-$Arch-setup.exe")
        if ($LASTEXITCODE -ne 0) { throw 'Installer signature verification failed' }
    }
} finally {
    $env:TQUE_SIGNTOOL_PATH=$oldTool; $env:TQUE_SIGN_TIMESTAMP_URL=$oldURL; $env:TQUE_SIGN_MACHINE_STORE=$oldStore
}
