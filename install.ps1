Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Repo = "thgrace/training-wheels"
$AssetName = "tw.exe"

function Fail {
    param([string]$Message)

    throw $Message
}

function Resolve-Version {
    if ([string]::IsNullOrWhiteSpace($env:TW_VERSION)) {
        return "latest"
    }

    if ($env:TW_VERSION -eq "latest" -or $env:TW_VERSION.StartsWith("v")) {
        return $env:TW_VERSION
    }

    return "v$($env:TW_VERSION)"
}

function Resolve-Arch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { Fail "unsupported architecture: $arch" }
    }
}

function Resolve-InstallDir {
    if (-not [string]::IsNullOrWhiteSpace($env:TW_INSTALL_DIR)) {
        return $env:TW_INSTALL_DIR
    }

    if ([string]::IsNullOrWhiteSpace($HOME)) {
        Fail "HOME is not set; set TW_INSTALL_DIR to continue"
    }

    return (Join-Path $HOME ".tw\bin")
}

function Verify-Checksum {
    param(
        [string]$FilePath,
        [string]$ChecksumPath,
        [string]$Asset
    )

    $expected = $null
    foreach ($line in Get-Content -Path $ChecksumPath) {
        if ($line -match '^(?<hash>[0-9a-fA-F]{64})\s+\*?(?<name>\S+)$' -and $Matches["name"] -eq $Asset) {
            $expected = $Matches["hash"].ToLowerInvariant()
            break
        }
    }

    if (-not $expected) {
        Write-Warning "checksum entry for $Asset was not found; skipping verification"
        return
    }

    $actual = (Get-FileHash -Algorithm SHA256 -Path $FilePath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        Fail "checksum mismatch for $Asset"
    }

    Write-Host "Verified checksum for $Asset."
}

function Ensure-UserPath {
    param([string]$InstallDir)

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @()
    if ($userPath) {
        $entries = $userPath -split ";" | Where-Object { $_ }
    }

    $normalizedInstallDir = $InstallDir.TrimEnd("\")
    foreach ($entry in $entries) {
        if ($entry.TrimEnd("\") -ieq $normalizedInstallDir) {
            return
        }
    }

    $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "Added $InstallDir to the user PATH. Restart your shell to pick it up."
}

if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
    Fail "install.ps1 supports Windows only"
}

$version = Resolve-Version
$arch = Resolve-Arch
$installDir = Resolve-InstallDir
$asset = "tw-windows-$arch.exe"

if ($version -eq "latest") {
    $releasePath = "latest/download"
    $versionLabel = "latest"
} else {
    $releasePath = "download/$version"
    $versionLabel = $version
}

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("tw-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

try {
    $binUrl = "https://github.com/$Repo/releases/$releasePath/$asset"
    $checksumUrl = "https://github.com/$Repo/releases/$releasePath/checksums.txt"
    $binTmp = Join-Path $tmpDir $asset
    $checksumTmp = Join-Path $tmpDir "checksums.txt"

    Write-Host "Installing $AssetName ($arch) from GitHub release $versionLabel..."
    Invoke-WebRequest -Uri $binUrl -OutFile $binTmp

    $haveChecksum = $false
    try {
        Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumTmp
        $haveChecksum = $true
    } catch {
        Write-Warning "could not download checksums.txt; continuing without verification"
    }
    if ($haveChecksum) {
        Verify-Checksum -FilePath $binTmp -ChecksumPath $checksumTmp -Asset $asset
    }

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item -Path $binTmp -Destination (Join-Path $installDir $AssetName) -Force
    Ensure-UserPath -InstallDir $installDir

    Write-Host "Installed $AssetName to $(Join-Path $installDir $AssetName)"
} finally {
    if (Test-Path $tmpDir) {
        Remove-Item -Path $tmpDir -Recurse -Force
    }
}
