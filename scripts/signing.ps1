# Shared signing configuration. CODESIGN_CERT uses the same subject-name
# convention as wtmux; thumbprints require an explicit selector.
function Get-TqueCertificateArguments {
    param([ValidateSet('SubjectName','Thumbprint')][string]$Selector = 'SubjectName')
    $certificateIdentity = $env:CODESIGN_CERT
    if ([string]::IsNullOrWhiteSpace($certificateIdentity)) {
        throw 'Set CODESIGN_CERT to the subject name of your code-signing certificate.'
    }
    if ($Selector -eq 'Thumbprint') {
        if ($certificateIdentity -notmatch '^[0-9a-fA-F]{40}$') {
            throw 'With -CertificateSelector Thumbprint, CODESIGN_CERT must contain a 40-digit SHA1 thumbprint.'
        }
        return @('/sha1', $certificateIdentity)
    }
    return @('/n', $certificateIdentity)
}

function Resolve-TqueSignTool {
    param([string]$Path)
    if ($Path) {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw 'SignToolPath must point to an existing file.' }
        return (Resolve-Path -LiteralPath $Path).Path
    }
    $sdkBin = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits/10/bin'
    $nativeArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    $architectures = @($nativeArch, 'x64') | Select-Object -Unique
    $sdkVersions = @(Get-ChildItem -LiteralPath $sdkBin -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^10\.\d+\.\d+\.\d+$' } |
        Sort-Object { [version]$_.Name } -Descending)
    foreach ($architecture in $architectures) {
        foreach ($sdkVersion in $sdkVersions) {
            $candidate = Join-Path $sdkVersion.FullName "$architecture/signtool.exe"
            if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
        }
    }
    $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    throw 'SignTool was not found. Install the Windows SDK or specify -SignToolPath.'
}
