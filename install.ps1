$Version = "v0.1.0"

# Detect architecture
$arch = if ($ENV:PROCESSOR_ARCHITECTURE -eq "AMD64") { "amd64" } else { "arm64" }
$binary = "csftp-windows-$arch.exe"
$url = "https://github.com/bongacat07/csftp-go/releases/download/$Version/$binary"

# Destination
$installDir = "$env:USERPROFILE\bin"
if (!(Test-Path $installDir)) { New-Item -ItemType Directory -Path $installDir }

# Download
Write-Host "Downloading $binary..."
Invoke-WebRequest -Uri $url -OutFile "$installDir\csftp.exe"

Write-Host "Installed to $installDir. Add to PATH if not already."
Write-Host "You can now run: csftp server or csftp client"
