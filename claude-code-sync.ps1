#Requires -Version 5.1
<#
.SYNOPSIS
    claude-code-sync - Secure Claude Code config sync across machines
.DESCRIPTION
    Cross-platform CLI to sync ~/.claude/ configs via GitHub with age encryption
.LINK
    https://github.com/felixisaac/claude-code-sync
#>

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Command = "help",

    [Parameter(Position = 1, ValueFromRemainingArguments)]
    [string[]]$Arguments
)

$ErrorActionPreference = "Stop"
$VERSION = "1.0.0"

# Directories
$CLAUDE_DIR = Join-Path $env:USERPROFILE ".claude"
$CLAUDE_JSON = Join-Path $env:USERPROFILE ".claude.json"
$SYNC_DIR = Join-Path $env:USERPROFILE ".claude-sync"
$CONFIG_FILE = Join-Path $SYNC_DIR "config"
$KEY_FILE = Join-Path $SYNC_DIR "identity.key"
$REPO_DIR = Join-Path $SYNC_DIR "repo"
$BACKUP_DIR = Join-Path $SYNC_DIR "backups"
$LOCK_FILE = Join-Path $SYNC_DIR ".lock"

# Patterns
$ENCRYPT_PATTERNS = @(
    "settings.json",
    "settings.local.json",
    "claude.json"
)

$EXCLUDE_PATTERNS = @(
    "plans",
    "*.log",
    "*.tmp",
    ".git",
    "*.local-backup-*",
    "sessionStorage",
    "*.cache",
    "projects",
    "local",
    "statsig",
    "history.jsonl",
    "todos"
)

#--- Utility Functions ---#

function Write-Info { param($Message) Write-Host "[INFO] $Message" -ForegroundColor Cyan }
function Write-Success { param($Message) Write-Host "[OK] $Message" -ForegroundColor Green }
function Write-Warn { param($Message) Write-Host "[WARN] $Message" -ForegroundColor Yellow }
function Write-Err { param($Message) Write-Host "[ERROR] $Message" -ForegroundColor Red }

function Test-Command {
    param([string]$Name)
    $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Test-Dependencies {
    $missing = @()
    if (-not (Test-Command "git")) { $missing += "git" }
    if (-not (Test-Command "age")) { $missing += "age" }

    if ($missing.Count -gt 0) {
        throw "Missing dependencies: $($missing -join ', '). Please install them first."
    }
}

function Get-Timestamp {
    Get-Date -Format "yyyyMMdd-HHmmss"
}

function Get-PublicKey {
    if (Test-Path $KEY_FILE) {
        $content = Get-Content $KEY_FILE -Raw
        if ($content -match "(age1[a-z0-9]+)") {
            return $Matches[1]
        }
    }
    return $null
}

function Test-ShouldEncrypt {
    param([string]$FilePath)

    $fileName = Split-Path $FilePath -Leaf
    $relPath = $FilePath.Replace("$CLAUDE_DIR\", "").Replace("$CLAUDE_DIR/", "")

    foreach ($pattern in $ENCRYPT_PATTERNS) {
        if ($fileName -eq $pattern -or $relPath -like $pattern) {
            return $true
        }
    }

    # Also encrypt claude.json
    if ($FilePath -eq $CLAUDE_JSON) {
        return $true
    }

    # Encrypt resources in skills
    if ($relPath -like "skills\*\resources\*" -or $relPath -like "skills/*/resources/*") {
        return $true
    }

    return $false
}

function Test-ShouldExclude {
    param([string]$FilePath)

    $fileName = Split-Path $FilePath -Leaf
    $relPath = $FilePath.Replace("$CLAUDE_DIR\", "").Replace("$CLAUDE_DIR/", "")

    foreach ($pattern in $EXCLUDE_PATTERNS) {
        if ($relPath -like "$pattern*" -or $fileName -like $pattern) {
            return $true
        }
    }
    return $false
}

function New-DirectoryIfNotExists {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Get-FileChecksum {
    param([string]$FilePath)
    (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash.ToLower()
}

#--- Commands ---#

function Invoke-Init {
    param([string]$RepoUrl)

    Write-Info "Initializing claude-code-sync..."
    Test-Dependencies

    # Create sync directory
    New-DirectoryIfNotExists $SYNC_DIR
    New-DirectoryIfNotExists $BACKUP_DIR

    # Generate age keypair if not exists
    if (Test-Path $KEY_FILE) {
        Write-Warn "Key already exists at $KEY_FILE"
        Write-Info "Public key: $(Get-PublicKey)"
    }
    else {
        Write-Info "Generating age keypair..."
        & age-keygen -o $KEY_FILE 2>&1 | Out-Host

        Write-Host ""
        Write-Host "========================================" -ForegroundColor Red
        Write-Host "   IMPORTANT: SAVE YOUR PRIVATE KEY!   " -ForegroundColor Red
        Write-Host "========================================" -ForegroundColor Red
        Write-Host ""
        Write-Host "Your private key (save this securely, you'll need it on other machines):"
        Write-Host ""
        Get-Content $KEY_FILE | Write-Host
        Write-Host ""
        Write-Host "This key will NOT be shown again!" -ForegroundColor Yellow
        Write-Host "Store it in a password manager or secure location." -ForegroundColor Yellow
        Write-Host ""
    }

    # Setup repo
    if ($RepoUrl) {
        if (Test-Path $REPO_DIR) {
            Write-Warn "Repo already exists at $REPO_DIR"
        }
        else {
            Write-Info "Cloning repo..."
            & git clone $RepoUrl $REPO_DIR
        }
        "repo_url=$RepoUrl" | Set-Content $CONFIG_FILE
    }
    else {
        if (-not (Test-Path $REPO_DIR)) {
            Write-Info "Creating local repo (you'll need to add a remote later)..."
            New-DirectoryIfNotExists $REPO_DIR
            & git -C $REPO_DIR init
            "# Claude Code Sync" | Set-Content (Join-Path $REPO_DIR "README.md")
            & git -C $REPO_DIR add README.md
            & git -C $REPO_DIR commit -m "Initial commit"
        }
        Write-Host ""
        Write-Info "No repo URL provided. To add a remote later:"
        Write-Host "  git -C $REPO_DIR remote add origin <your-repo-url>"
        Write-Host "  claude-code-sync push"
    }

    Write-Success "Initialization complete!"
}

function Invoke-Push {
    Test-Dependencies

    if (-not (Test-Path $KEY_FILE)) {
        throw "Not initialized. Run 'claude-code-sync init' first."
    }

    if (-not (Test-Path $CLAUDE_DIR)) {
        throw "No ~/.claude directory found. Nothing to sync."
    }

    Write-Info "Syncing files to repo..."

    $publicKey = Get-PublicKey
    $count = 0

    # Process ~/.claude directory
    Get-ChildItem -Path $CLAUDE_DIR -Recurse -File | ForEach-Object {
        $file = $_.FullName
        $relPath = $file.Replace("$CLAUDE_DIR\", "").Replace("$CLAUDE_DIR/", "")
        $dest = Join-Path $REPO_DIR $relPath

        # Skip excluded files
        if (Test-ShouldExclude $file) {
            return
        }

        # Create parent directory
        New-DirectoryIfNotExists (Split-Path $dest -Parent)

        # Encrypt or copy
        if (Test-ShouldEncrypt $file) {
            Write-Info "Encrypting: $relPath"
            & age -e -r $publicKey -o "$dest.age" $file
        }
        else {
            Write-Info "Copying: $relPath"
            Copy-Item $file $dest -Force
        }
        $script:count++
    }

    # Also sync ~/.claude.json if it exists
    if (Test-Path $CLAUDE_JSON) {
        Write-Info "Encrypting: claude.json"
        & age -e -r $publicKey -o (Join-Path $REPO_DIR "claude.json.age") $CLAUDE_JSON
        $count++
    }

    # Generate manifest
    Write-Info "Generating manifest..."
    $manifest = Join-Path $REPO_DIR ".sync-manifest"
    $manifestContent = @("# claude-code-sync manifest - $(Get-Date -Format 'o')", "# Format: checksum  path")

    Get-ChildItem -Path $REPO_DIR -Recurse -File | ForEach-Object {
        $relPath = $_.FullName.Replace("$REPO_DIR\", "").Replace("$REPO_DIR/", "")
        if ($relPath -like ".git*" -or $relPath -eq ".sync-manifest") { return }
        $checksum = Get-FileChecksum $_.FullName
        $manifestContent += "$checksum  $relPath"
    }
    $manifestContent | Set-Content $manifest

    # Git commit and push
    Write-Info "Committing changes..."
    & git -C $REPO_DIR add -A

    $diff = & git -C $REPO_DIR diff --cached --quiet 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Info "No changes to commit."
    }
    else {
        & git -C $REPO_DIR commit -m "Sync $(Get-Timestamp)"

        $remotes = & git -C $REPO_DIR remote
        if ($remotes -contains "origin") {
            Write-Info "Pushing to remote..."
            & git -C $REPO_DIR push origin HEAD
            Write-Success "Pushed $count files to remote."
        }
        else {
            Write-Warn "No remote configured. Changes committed locally only."
            Write-Info "Add a remote with: git -C $REPO_DIR remote add origin <url>"
        }
    }

    Write-Success "Push complete!"
}

function Invoke-Pull {
    Test-Dependencies

    if (-not (Test-Path $KEY_FILE)) {
        throw "Not initialized. Run 'claude-code-sync init' or 'claude-code-sync import-key' first."
    }

    if (-not (Test-Path $REPO_DIR)) {
        throw "No repo found. Run 'claude-code-sync init <repo-url>' first."
    }

    # Pull from remote
    $remotes = & git -C $REPO_DIR remote
    if ($remotes -contains "origin") {
        Write-Info "Pulling from remote..."
        & git -C $REPO_DIR pull origin HEAD
    }

    # Backup current config
    if (Test-Path $CLAUDE_DIR) {
        $backupPath = Join-Path $BACKUP_DIR "backup-$(Get-Timestamp).zip"
        Write-Info "Backing up current config to $backupPath..."
        Compress-Archive -Path $CLAUDE_DIR, $CLAUDE_JSON -DestinationPath $backupPath -Force -ErrorAction SilentlyContinue

        # Keep only last 5 backups
        Get-ChildItem $BACKUP_DIR -Filter "backup-*.zip" | Sort-Object LastWriteTime -Descending | Select-Object -Skip 5 | Remove-Item -Force
    }

    New-DirectoryIfNotExists $CLAUDE_DIR

    # Process files from repo
    Write-Info "Restoring files..."
    $count = 0

    Get-ChildItem -Path $REPO_DIR -Recurse -File | ForEach-Object {
        $file = $_.FullName
        $relPath = $file.Replace("$REPO_DIR\", "").Replace("$REPO_DIR/", "")

        # Skip git and manifest
        if ($relPath -like ".git*" -or $relPath -eq ".sync-manifest" -or $relPath -eq "README.md") { return }

        $actualRelPath = $relPath
        $dest = $null

        # Handle encrypted files
        if ($relPath -like "*.age") {
            $actualRelPath = $relPath -replace "\.age$", ""

            # Special case for claude.json
            if ($actualRelPath -eq "claude.json") {
                $dest = $CLAUDE_JSON
            }
            else {
                $dest = Join-Path $CLAUDE_DIR $actualRelPath
            }

            # Backup existing file if different
            if (Test-Path $dest) {
                $existingContent = Get-Content $dest -Raw -ErrorAction SilentlyContinue
                $newContent = & age -d -i $KEY_FILE $file 2>$null
                if ($existingContent -ne $newContent) {
                    $backupName = "$dest.local-backup-$(Get-Timestamp)"
                    Write-Warn "Conflict: backing up $actualRelPath to $backupName"
                    Copy-Item $dest $backupName -Force
                }
            }

            Write-Info "Decrypting: $actualRelPath"
            New-DirectoryIfNotExists (Split-Path $dest -Parent)
            & age -d -i $KEY_FILE -o $dest $file
        }
        else {
            $dest = Join-Path $CLAUDE_DIR $relPath

            # Backup existing file if different
            if (Test-Path $dest) {
                $existingHash = Get-FileChecksum $dest
                $newHash = Get-FileChecksum $file
                if ($existingHash -ne $newHash) {
                    $backupName = "$dest.local-backup-$(Get-Timestamp)"
                    Write-Warn "Conflict: backing up $relPath to $backupName"
                    Copy-Item $dest $backupName -Force
                }
            }

            Write-Info "Copying: $relPath"
            New-DirectoryIfNotExists (Split-Path $dest -Parent)
            Copy-Item $file $dest -Force
        }
        $script:count++
    }

    Write-Success "Pull complete! Restored $count files."
}

function Invoke-Status {
    Test-Dependencies

    if (-not (Test-Path $REPO_DIR)) {
        throw "No repo found. Run 'claude-code-sync init' first."
    }

    Write-Host "=== claude-code-sync status ===" -ForegroundColor Cyan
    Write-Host ""

    # Check remote status
    $remotes = & git -C $REPO_DIR remote
    if ($remotes -contains "origin") {
        & git -C $REPO_DIR fetch origin 2>$null
        $localCommit = & git -C $REPO_DIR rev-parse HEAD 2>$null
        $remoteCommit = & git -C $REPO_DIR rev-parse origin/HEAD 2>$null

        if ($localCommit -eq $remoteCommit) {
            Write-Host "Remote: " -NoNewline; Write-Host "Up to date" -ForegroundColor Green
        }
        else {
            Write-Host "Remote: " -NoNewline; Write-Host "Out of sync (local: $($localCommit.Substring(0,7)), remote: $($remoteCommit.Substring(0,7)))" -ForegroundColor Yellow
        }
    }
    else {
        Write-Host "Remote: " -NoNewline; Write-Host "Not configured" -ForegroundColor Yellow
    }

    Write-Host ""
    Write-Host "Local files in ~/.claude:"

    if (Test-Path $CLAUDE_DIR) {
        Get-ChildItem -Path $CLAUDE_DIR -Recurse -File | ForEach-Object {
            $relPath = $_.FullName.Replace("$CLAUDE_DIR\", "")
            if (Test-ShouldExclude $_.FullName) {
                Write-Host "  [excluded] $relPath" -ForegroundColor Yellow
            }
            elseif (Test-ShouldEncrypt $_.FullName) {
                Write-Host "  [encrypted] $relPath" -ForegroundColor Cyan
            }
            else {
                Write-Host "  [plain] $relPath" -ForegroundColor Green
            }
        }
    }
    else {
        Write-Host "  (none)"
    }

    if (Test-Path $CLAUDE_JSON) {
        Write-Host "  [encrypted] ~/.claude.json" -ForegroundColor Cyan
    }

    Write-Host ""
    Write-Host "Repo files in $REPO_DIR`:"

    Get-ChildItem -Path $REPO_DIR -Recurse -File | ForEach-Object {
        $relPath = $_.FullName.Replace("$REPO_DIR\", "")
        if ($relPath -like ".git*") { return }
        if ($relPath -like "*.age") {
            Write-Host "  [encrypted] $relPath" -ForegroundColor Cyan
        }
        else {
            Write-Host "  [plain] $relPath" -ForegroundColor Green
        }
    }
}

function Invoke-ImportKey {
    New-DirectoryIfNotExists $SYNC_DIR

    if (Test-Path $KEY_FILE) {
        Write-Warn "Key already exists at $KEY_FILE"
        $confirm = Read-Host "Overwrite? (y/N)"
        if ($confirm -notmatch "^[Yy]$") {
            throw "Aborted."
        }
    }

    Write-Host "Paste your age private key (starts with AGE-SECRET-KEY-):"
    Write-Host "Press Ctrl+Z then Enter when done."
    Write-Host ""

    $keyContent = @()
    while ($true) {
        $line = Read-Host
        if ($null -eq $line) { break }
        $keyContent += $line
    }
    $keyString = $keyContent -join "`n"

    # Validate key format
    if ($keyString -notmatch "AGE-SECRET-KEY-") {
        throw "Invalid key format. Key must contain AGE-SECRET-KEY-"
    }

    $keyString | Set-Content $KEY_FILE -NoNewline

    Write-Success "Key imported successfully!"
    Write-Info "Public key: $(Get-PublicKey)"
}

function Invoke-ExportKey {
    if (-not (Test-Path $KEY_FILE)) {
        throw "No key found. Run 'claude-code-sync init' first."
    }

    Write-Host ""
    Write-Host "=== Your Private Key ===" -ForegroundColor Yellow
    Write-Host ""
    Get-Content $KEY_FILE | Write-Host
    Write-Host ""
    Write-Host "Keep this secure!" -ForegroundColor Yellow
}

function Invoke-Verify {
    Test-Dependencies

    $manifestPath = Join-Path $REPO_DIR ".sync-manifest"
    if (-not (Test-Path $manifestPath)) {
        throw "No manifest found. Run 'claude-code-sync push' first."
    }

    Write-Info "Verifying file integrity..."
    $errors = 0

    Get-Content $manifestPath | ForEach-Object {
        if ($_ -match "^#" -or [string]::IsNullOrWhiteSpace($_)) { return }

        $parts = $_ -split "\s+", 2
        $expectedChecksum = $parts[0]
        $filePath = $parts[1].Trim()

        $fullPath = Join-Path $REPO_DIR $filePath

        if (-not (Test-Path $fullPath)) {
            Write-Err "Missing: $filePath"
            $script:errors++
            return
        }

        $actualChecksum = Get-FileChecksum $fullPath

        if ($expectedChecksum -ne $actualChecksum) {
            Write-Err "Checksum mismatch: $filePath"
            $script:errors++
        }
        else {
            Write-Success "OK: $filePath"
        }
    }

    Write-Host ""
    if ($errors -eq 0) {
        Write-Success "All files verified!"
    }
    else {
        throw "$errors file(s) failed verification."
    }
}

function Show-Version {
    Write-Host "claude-code-sync v$VERSION"
}

function Show-Help {
    @"
claude-code-sync - Secure Claude Code config sync across machines

USAGE:
    claude-code-sync <command> [options]

COMMANDS:
    init [repo-url]    Initialize sync (generate keys, clone/create repo)
    push               Encrypt and push configs to GitHub
    pull               Pull and decrypt configs from GitHub
    status             Show sync status
    import-key         Import private key on new machine
    export-key         Display private key for backup
    verify             Verify file integrity
    version            Show version
    help               Show this help

FIRST TIME SETUP:
    1. claude-code-sync init
    2. Save the displayed private key securely!
    3. Create a private GitHub repo
    4. git -C ~/.claude-sync/repo remote add origin <repo-url>
    5. claude-code-sync push

NEW MACHINE SETUP:
    1. Install: irm https://raw.githubusercontent.com/felixisaac/claude-code-sync/main/install.ps1 | iex
    2. claude-code-sync import-key  (paste your saved private key)
    3. claude-code-sync init <repo-url>
    4. claude-code-sync pull

MORE INFO:
    https://github.com/felixisaac/claude-code-sync
"@
}

#--- Main ---#

switch ($Command.ToLower()) {
    "init"       { Invoke-Init $Arguments[0] }
    "push"       { Invoke-Push }
    "pull"       { Invoke-Pull }
    "status"     { Invoke-Status }
    "import-key" { Invoke-ImportKey }
    "export-key" { Invoke-ExportKey }
    "verify"     { Invoke-Verify }
    "version"    { Show-Version }
    "-v"         { Show-Version }
    "--version"  { Show-Version }
    "help"       { Show-Help }
    "-h"         { Show-Help }
    "--help"     { Show-Help }
    default      { throw "Unknown command: $Command. Run 'claude-code-sync help' for usage." }
}
