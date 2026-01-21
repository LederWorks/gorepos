#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Common test script for GoRepos testing environments

.DESCRIPTION
    This script provides a comprehensive test suite for GoRepos functionality.
    It can be called by specific test environment scripts with different configurations.

.PARAMETER ConfigFile
    Path to the configuration file

.PARAMETER BasePath
    Base path for repositories

.PARAMETER TestName
    Name of the test environment (for display purposes)

.PARAMETER Setup
    Run gorepos setup --force

.PARAMETER SkipHelp
    Skip help command tests

.PARAMETER SkipValidate
    Skip gorepos validate

.PARAMETER SkipStatus
    Skip gorepos status

.PARAMETER SkipGraph
    Skip gorepos graph

.PARAMETER Groups
    Run gorepos groups

.PARAMETER Clone
    Run gorepos clone

.PARAMETER Update
    Run gorepos update

.PARAMETER VerboseOutput
    Enable verbose output for all commands
#>

param(
    [Parameter(Mandatory = $true)]
    [string]$ConfigFile,
    
    [Parameter(Mandatory = $true)]
    [string]$BasePath,
    
    [Parameter(Mandatory = $true)]
    [string]$TestName,
    
    [switch]$Setup,
    [switch]$SkipHelp,
    [switch]$SkipValidate,
    [switch]$SkipStatus,
    [switch]$SkipGraph,
    [switch]$Groups,
    [switch]$Clone,
    [switch]$Update,
    [switch]$VerboseOutput
)

# Color functions
function Write-Info {
    param($Message)
    Write-Host "ℹ️  $Message" -ForegroundColor Cyan
}

function Write-Success {
    param($Message)
    Write-Host "✅ $Message" -ForegroundColor Green
}

function Write-Error {
    param($Message)
    Write-Host "❌ $Message" -ForegroundColor Red
}

function Write-Warning {
    param($Message)
    Write-Host "⚠️  $Message" -ForegroundColor Yellow
}

function Write-Section {
    param($Message)
    Write-Host ""
    Write-Host "🔵 $Message" -ForegroundColor Blue
    Write-Host "=" * ($Message.Length + 3)
}

function Run-GoReposCommand {
    param(
        [string]$Command,
        [string]$Description,
        [switch]$SkipConfigFlag,
        [switch]$ExpectFailure
    )
    
    Write-Info "Running: $Description"
    
    $Args = $Command.Split(' ')
    
    # Add verbose flag if requested
    if ($VerboseOutput -and $Command -notmatch "--verbose") {
        $Args += "--verbose"
    }
    
    # Add config file unless skipped or it's a help command
    if (-not $SkipConfigFlag -and $Command -notmatch "help") {
        $Args += "--config"
        $Args += $ConfigFile
    }
    
    Write-Host "  Command: gorepos $($Args -join ' ')" -ForegroundColor Gray
    
    if ($VerboseOutput) {
        Write-Host "  Output:" -ForegroundColor Gray
        Write-Host "  ========" -ForegroundColor Gray
        # Set UTF-8 encoding to properly handle Unicode tree characters
        $PreviousEncoding = [Console]::OutputEncoding
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
        $Output = & gorepos @Args 2>&1
        [Console]::OutputEncoding = $PreviousEncoding
        if ($Output) {
            $Output | ForEach-Object { Write-Host "  $_" }
        }
        Write-Host "  ========" -ForegroundColor Gray
    } else {
        & gorepos @Args *> $null
    }
    $ExitCode = $LASTEXITCODE
    
    if ($ExpectFailure) {
        if ($ExitCode -ne 0) {
            Write-Success "  Expected failure - Command failed as expected"
        } else {
            Write-Warning "  Expected failure but command succeeded"
        }
    } else {
        if ($ExitCode -eq 0) {
            Write-Success "  Command completed successfully"
        } else {
            Write-Error "  Command failed with exit code: $ExitCode"
            return $false
        }
    }
    
    return $true
}

# Get project root and binary info
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$VersionFile = Join-Path $ProjectRoot "dist" "VERSION"

Write-Section "$TestName Test Suite"

# Read version from dist/VERSION
if (-not (Test-Path $VersionFile)) {
    Write-Error "VERSION file not found: $VersionFile"
    Write-Info "Run build script first: .\scripts\build.ps1"
    exit 1
}

$Version = Get-Content $VersionFile -Raw | ForEach-Object { $_.Trim() }
$BinaryPath = Join-Path $ProjectRoot "dist" "windows-amd64" "gorepos-$Version" "gorepos.exe"

Write-Info "Version: $Version"
Write-Info "Binary: $BinaryPath"
Write-Info "Config File: $ConfigFile"
Write-Info "Base Path: $BasePath"

# Check if binary exists
if (-not (Test-Path $BinaryPath)) {
    Write-Error "Binary not found: $BinaryPath"
    Write-Info "Available binaries:"
    Get-ChildItem (Join-Path $ProjectRoot "dist") -Recurse -Name "gorepos.exe" | ForEach-Object {
        Write-Info "  $_"
    }
    exit 1
}

# Add binary directory to PATH for this session
$BinaryDir = Split-Path $BinaryPath -Parent
$env:PATH = "$BinaryDir;$env:PATH"
Write-Info "Added to PATH: $BinaryDir"

$FailedTests = 0
$TotalTests = 0

# 1. Setup (if requested)
if ($Setup) {
    Write-Section "Setup Phase"
    $TotalTests++
    
    Write-Info "Creating/updating test environment..."
    
    # Create directory if it doesn't exist
    $ConfigDir = Split-Path $ConfigFile -Parent
    if (-not (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }
    
    $SetupArgs = @("setup")
    
    # Use custom path for grtest2, default discovery for grtest1
    if ($ConfigFile -notmatch "OneDrive.*Documents.*gorepos") {
        $SetupArgs += "--path"
        $SetupArgs += $ConfigFile
    }
    
    $SetupArgs += "--base-path"
    $SetupArgs += $BasePath
    $SetupArgs += "--includes"
    $SetupArgs += "C:\Data\GIT\GitHub\gorepos\gorepos-config\gorepos.yaml"
    $SetupArgs += "--force"
    
    Write-Host "  Command: gorepos $($SetupArgs -join ' ')" -ForegroundColor Gray
    
    if ($VerboseOutput) {
        Write-Host "  Output:" -ForegroundColor Gray
        Write-Host "  ========" -ForegroundColor Gray
        # Set UTF-8 encoding to properly handle Unicode characters
        $PreviousEncoding = [Console]::OutputEncoding
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
        $Output = & gorepos @SetupArgs 2>&1
        [Console]::OutputEncoding = $PreviousEncoding
        if ($Output) {
            $Output | ForEach-Object { Write-Host "  $_" }
        }
        Write-Host "  ========" -ForegroundColor Gray
    } else {
        & gorepos @SetupArgs *> $null
    }
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Setup completed successfully"
    } else {
        Write-Error "Setup failed"
        $FailedTests++
    }
}

# 2. Help commands (unless skipped)
if (-not $SkipHelp) {
    Write-Section "Help Commands"
    
    $HelpCommands = @(
        @{ Command = "--help"; Description = "Main help" },
        @{ Command = "status --help"; Description = "Status command help" },
        @{ Command = "update --help"; Description = "Update command help" },
        @{ Command = "validate --help"; Description = "Validate command help" },
        @{ Command = "clone --help"; Description = "Clone command help" },
        @{ Command = "groups --help"; Description = "Groups command help" },
        @{ Command = "graph --help"; Description = "Graph command help" },
        @{ Command = "setup --help"; Description = "Setup command help" }
    )
    
    foreach ($HelpCmd in $HelpCommands) {
        $TotalTests++
        if (-not (Run-GoReposCommand -Command $HelpCmd.Command -Description $HelpCmd.Description -SkipConfigFlag)) {
            $FailedTests++
        }
    }
}

# 3. Validate (unless skipped)
if (-not $SkipValidate) {
    Write-Section "Configuration Validation"
    $TotalTests++
    if (-not (Run-GoReposCommand -Command "validate" -Description "Configuration validation")) {
        $FailedTests++
    }
}

# 4. Status (unless skipped)
if (-not $SkipStatus) {
    Write-Section "Repository Status"
    $TotalTests++
    if (-not (Run-GoReposCommand -Command "status" -Description "Repository status check")) {
        $FailedTests++
    }
}

# 5. Graph (unless skipped)
if (-not $SkipGraph) {
    Write-Section "Configuration Graph"
    $TotalTests++
    if (-not (Run-GoReposCommand -Command "graph" -Description "Configuration graph visualization")) {
        $FailedTests++
    }
}

# 6. Groups (if requested)
if ($Groups) {
    Write-Section "Repository Groups"
    $TotalTests++
    if (-not (Run-GoReposCommand -Command "groups" -Description "Repository groups listing")) {
        $FailedTests++
    }
}

# 7. Clone (if requested)
if ($Clone) {
    Write-Section "Repository Clone"
    $TotalTests++
    if (-not (Run-GoReposCommand -Command "clone" -Description "Clone missing repositories")) {
        $FailedTests++
    }
}

# 8. Update (if requested)
if ($Update) {
    Write-Section "Repository Update"
    $TotalTests++
    if (-not (Run-GoReposCommand -Command "update" -Description "Update repositories")) {
        $FailedTests++
    }
}

# Summary
Write-Section "Test Results Summary"
Write-Info "Total tests run: $TotalTests"
if ($FailedTests -eq 0) {
    Write-Success "All tests passed! ✨"
} else {
    Write-Error "Failed tests: $FailedTests"
    Write-Warning "Passed tests: $($TotalTests - $FailedTests)"
}

Write-Info "Test suite completed for: $TestName"

exit $FailedTests