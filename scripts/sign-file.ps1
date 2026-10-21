[CmdletBinding()]
param([Parameter(Mandatory)][string]$Path)
$ErrorActionPreference = 'Stop'
try {
    # Identity format remains a deployment decision. Support explicit store thumbprints;
    # do not guess a certificate subject or import a PFX into the user's store.
    $identity = $env:CODESIGN_CERT
    if (-not $identity -or $identity -notmatch '^[0-9a-fA-F]{40}$') {
        throw 'CODESIGN_CERT must be a certificate-store SHA1 thumbprint. Other formats require explicit configuration.'
    }
    if (-not $env:TQUE_SIGNTOOL_PATH -or -not (Test-Path -LiteralPath $env:TQUE_SIGNTOOL_PATH)) { throw 'SignTool path is missing' }
    $timestamp = $env:TQUE_SIGN_TIMESTAMP_URL
    if (-not $timestamp -or $timestamp -notmatch '^https?://') { throw 'RFC3161 timestamp URL is required' }
    $arguments = @('sign','/q','/fd','SHA256','/td','SHA256','/tr',$timestamp,'/sha1',$identity)
    if ($env:TQUE_SIGN_MACHINE_STORE -eq '1') { $arguments += '/sm' }
    $arguments += (Resolve-Path -LiteralPath $Path).Path
    & $env:TQUE_SIGNTOOL_PATH @arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Signing failed' }
    & $env:TQUE_SIGNTOOL_PATH verify /q /pa $Path | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Signature verification failed' }
} catch {
    Write-Error $_
    exit 1
}
