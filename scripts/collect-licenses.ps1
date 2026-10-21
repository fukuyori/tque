[CmdletBinding()]
param([Parameter(Mandatory)][string]$OutputDirectory)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$OutputDirectory = $PSCmdlet.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
Push-Location $repo
try {
    & go mod download all
    if ($LASTEXITCODE -ne 0) { throw 'Dependency download failed' }
    $modules = & go list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' all
    if ($LASTEXITCODE -ne 0) { throw 'Module listing failed' }
    New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
    $index = [Collections.Generic.List[string]]::new()
    foreach ($entry in $modules) {
        $parts = $entry.Split('|')
        if ($parts[0] -eq 'tque') { continue }
        $files = @(Get-ChildItem -LiteralPath $parts[2] -File -Recurse | Where-Object { $_.Name -match '^(LICENSE|COPYING|NOTICE|COPYRIGHT)' })
        if ($files.Count -eq 0) { throw "No license found for module $($parts[0])" }
        $folder = Join-Path $OutputDirectory (($parts[0] + '@' + $parts[1]) -replace '[^a-zA-Z0-9._@-]', '_')
        New-Item -ItemType Directory -Force -Path $folder | Out-Null
        foreach ($file in $files) {
            $relative = $file.FullName.Substring($parts[2].TrimEnd('\','/').Length).TrimStart('\','/')
            $destination = Join-Path $folder $relative
            New-Item -ItemType Directory -Force -Path (Split-Path $destination -Parent) | Out-Null
            Copy-Item -LiteralPath $file.FullName -Destination $destination -Force
        }
        $index.Add("$($parts[0]) $($parts[1])")
    }
    # The statically linked Go runtime is BSD licensed too.
    $goRootPath = & go env GOROOT
    if ($LASTEXITCODE -ne 0) { throw 'GOROOT lookup failed' }
    Copy-Item -LiteralPath (Join-Path $goRootPath 'LICENSE') -Destination (Join-Path $OutputDirectory 'GO-LICENSE') -Force
    $index | Set-Content -LiteralPath (Join-Path $OutputDirectory 'MODULES.txt') -Encoding utf8
} finally { Pop-Location }
