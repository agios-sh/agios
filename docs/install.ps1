# AGI OS (agios) installer for Windows
# Usage: irm https://agios.sh/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = "agios-sh/agios"
$BinaryName = "agios"
$InstallDir = "$env:USERPROFILE\.agios\bin"

function Main {
    Detect-Platform
    Fetch-LatestVersion
    Download-AndInstall
    Add-ToPath
    Verify-Installation
}

function Detect-Platform {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { $script:Arch = "amd64" }
        "Arm64" { $script:Arch = "arm64" }
        default {
            Write-Error "Unsupported architecture: $arch (supported: amd64, arm64)"
            exit 1
        }
    }
    Write-Host "==> Detected platform: windows/$script:Arch" -ForegroundColor Green
}

function Fetch-LatestVersion {
    Write-Host "==> Fetching latest release..." -ForegroundColor Green
    $releaseUrl = "https://api.github.com/repos/$Repo/releases/latest"

    try {
        $release = Invoke-RestMethod -Uri $releaseUrl -UseBasicParsing
        $script:Tag = $release.tag_name
    }
    catch {
        Write-Error "Failed to fetch latest release from GitHub: $_"
        exit 1
    }

    if (-not $script:Tag) {
        Write-Error "Could not determine latest release version"
        exit 1
    }
    Write-Host "==> Latest version: $script:Tag" -ForegroundColor Green
}

function Download-AndInstall {
    $archive = "${BinaryName}_windows_${script:Arch}.zip"
    $downloadUrl = "https://github.com/$Repo/releases/download/$script:Tag/$archive"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        $zipPath = Join-Path $tmpDir $archive

        Write-Host "==> Downloading $archive..." -ForegroundColor Green
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
        }
        catch {
            Write-Error "Failed to download ${downloadUrl}: $_"
            exit 1
        }

        Write-Host "==> Extracting..." -ForegroundColor Green
        Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

        $binaryPath = Join-Path $tmpDir "${BinaryName}.exe"
        if (-not (Test-Path $binaryPath)) {
            Write-Error "Binary not found in archive"
            exit 1
        }

        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        Write-Host "==> Installing to $InstallDir\${BinaryName}.exe..." -ForegroundColor Green
        Copy-Item -Path $binaryPath -Destination (Join-Path $InstallDir "${BinaryName}.exe") -Force
    }
    finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Add-ToPath {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        Write-Host "==> Adding $InstallDir to user PATH..." -ForegroundColor Green
        $newPath = "$InstallDir;$userPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$InstallDir;$env:Path"
    }
}

function Verify-Installation {
    $exePath = Join-Path $InstallDir "${BinaryName}.exe"
    if (Test-Path $exePath) {
        try {
            $installedVersion = & $exePath --version 2>$null
        }
        catch {
            $installedVersion = "unknown"
        }
        Write-Host "==> Successfully installed ${BinaryName} ${installedVersion}" -ForegroundColor Green
        Write-Host "==> Run 'agios init' to get started" -ForegroundColor Green

        if ($userPath -notlike "*$InstallDir*") {
            Write-Host "warning: Restart your terminal for PATH changes to take effect" -ForegroundColor Yellow
        }
    }
    else {
        Write-Error "Installation failed: binary not found at $exePath"
        exit 1
    }
}

Main
