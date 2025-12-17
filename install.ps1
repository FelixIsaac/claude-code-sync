#Requires -Version 5.1
# claude-code-sync installer for Windows

$ErrorActionPreference = "Stop"

$Repo = "felixisaac/claude-code-sync"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:USERPROFILE ".local\bin" }
$ScriptName = "claude-code-sync.ps1"

Write-Host "Installing claude-code-sync..." -ForegroundColor Cyan

# Check for age
$ageCmd = Get-Command age -ErrorAction SilentlyContinue
if (-not $ageCmd) {
    Write-Host ""
    Write-Host "age is required but not installed. Install it first:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  scoop install age" -ForegroundColor White
    Write-Host "  # or"
    Write-Host "  winget install FiloSottile.age" -ForegroundColor White
    Write-Host ""
    Write-Host "See https://github.com/FiloSottile/age#installation" -ForegroundColor Gray
    Write-Host ""
    exit 1
}

# Check for git
$gitCmd = Get-Command git -ErrorAction SilentlyContinue
if (-not $gitCmd) {
    Write-Host "git is required but not installed. Please install git first." -ForegroundColor Red
    exit 1
}

# Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Download script
Write-Host "Downloading from GitHub..."
$url = "https://raw.githubusercontent.com/$Repo/main/$ScriptName"
$destPath = Join-Path $InstallDir $ScriptName
Invoke-WebRequest -Uri $url -OutFile $destPath

# Create batch wrapper for easier invocation
$batchPath = Join-Path $InstallDir "claude-code-sync.cmd"
@"
@echo off
powershell -ExecutionPolicy Bypass -File "%~dp0$ScriptName" %*
"@ | Set-Content $batchPath

# Check if in PATH
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath -notlike "*$InstallDir*") {
    Write-Host ""
    Write-Host "Adding $InstallDir to your PATH..." -ForegroundColor Yellow

    $newPath = "$currentPath;$InstallDir"
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")

    Write-Host "PATH updated. Please restart your terminal for changes to take effect." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Installed successfully to $destPath" -ForegroundColor Green
Write-Host ""
Write-Host "Get started:" -ForegroundColor Cyan
Write-Host "  claude-code-sync init"
Write-Host ""
Write-Host "For help:" -ForegroundColor Cyan
Write-Host "  claude-code-sync help"
