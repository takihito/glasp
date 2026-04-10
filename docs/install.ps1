# glasp installer for Windows
# Usage: irm https://takihito.github.io/glasp/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = "takihito/glasp"
$InstallDir = if ($env:GLASP_INSTALL_DIR) { $env:GLASP_INSTALL_DIR } else { "$env:LOCALAPPDATA\glasp\bin" }

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit systems are not supported"; exit 1
}

# Get latest version
Write-Host "Fetching latest version..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name
Write-Host "Latest version: $Version"

# Download
$Artifact = "glasp_${Version}_windows_${Arch}.zip"
$Checksums = "checksums.txt"
$BaseUrl = "https://github.com/$Repo/releases/download/$Version"
$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "glasp-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "Downloading $Artifact..."
    Invoke-WebRequest -Uri "$BaseUrl/$Artifact" -OutFile "$TmpDir\$Artifact"
    Invoke-WebRequest -Uri "$BaseUrl/$Checksums" -OutFile "$TmpDir\$Checksums"

    # Verify checksum
    Write-Host "Verifying checksum..."
    $Expected = (Get-Content "$TmpDir\$Checksums" | Select-String "  $Artifact$").ToString().Split(" ")[0]
    $Actual = (Get-FileHash "$TmpDir\$Artifact" -Algorithm SHA256).Hash.ToLower()
    if ($Expected -ne $Actual) {
        Write-Error "Checksum mismatch: expected $Expected, got $Actual"
        exit 1
    }

    # Extract and install
    Write-Host "Installing to $InstallDir..."
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Expand-Archive -Path "$TmpDir\$Artifact" -DestinationPath $TmpDir -Force
    Copy-Item "$TmpDir\glasp.exe" -Destination "$InstallDir\glasp.exe" -Force

    # Add to PATH if not already present
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to PATH (restart your terminal to apply)"
    }

    Write-Host ""
    Write-Host "glasp $Version installed successfully!"
    Write-Host "Run 'glasp version' to verify."
} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
