#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Build script for GoRepos project (PowerShell)

.DESCRIPTION
    This script builds the GoRepos binary with various options for different platforms.
    It handles cross-compilation, versioning, and output organization.

.PARAMETER Target
    Target platform for cross-compilation (default: current platform)
    Valid values: windows, linux, darwin, all

.PARAMETER Arch
    Target architecture (default: current architecture)
    Valid values: amd64, arm64, all

.PARAMETER Output
    Output directory for built binaries (default: ./dist)

.PARAMETER Version
    Version string to embed in the binary (default: git describe or "dev")

.PARAMETER Clean
    Clean the output directory before building

.PARAMETER Test
    Run tests before building

.PARAMETER Verbose
    Enable verbose output

.EXAMPLE
    .\scripts\build.ps1
    Build for current platform

.EXAMPLE
    .\scripts\build.ps1 -Target all -Arch amd64 -Clean
    Build for all platforms with amd64 architecture and clean first

.EXAMPLE
    .\scripts\build.ps1 -Target windows -Test -Verbose
    Build for Windows with tests and verbose output
#>

param(
    [string]$Target = "all",
    [string]$Arch = "all",
    [string]$Output = "dist",
    [string]$Version = "",
    [switch]$ContentHash,
    [switch]$Clean,
    [switch]$Test,
    [switch]$Verbose
)

# Set error action preference
$ErrorActionPreference = "Stop"

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

# Content-based versioning function
function Get-ContentHash {
    param([string]$BaseDir = ".")
    
    Write-Info "Calculating content hash for Go source files..."
    
    # Find all Go files and relevant config files
    $SourceFiles = @()
    $SourceFiles += Get-ChildItem -Path $BaseDir -Filter "*.go" -Recurse | Where-Object { $_.FullName -notmatch "[\\/]vendor[\\/]" }
    $SourceFiles += Get-ChildItem -Path $BaseDir -Filter "go.mod"
    $SourceFiles += Get-ChildItem -Path $BaseDir -Filter "go.sum" -ErrorAction SilentlyContinue
    
    # Calculate hash of all file contents
    $AllContent = ""
    foreach ($File in ($SourceFiles | Sort-Object FullName)) {
        $RelativePath = $File.FullName.Replace("$BaseDir\", "").Replace("\", "/")
        $Content = Get-Content $File.FullName -Raw -ErrorAction SilentlyContinue
        $AllContent += "$RelativePath`:$Content"
    }
    
    # Create SHA256 hash and take first 8 characters
    $StringAsStream = [System.IO.MemoryStream]::new()
    $Data = [System.Text.Encoding]::UTF8.GetBytes($AllContent)
    $StringAsStream.Write($Data, 0, $Data.Length)
    $StringAsStream.Position = 0
    $Hash = Get-FileHash -InputStream $StringAsStream -Algorithm SHA256
    $ShortHash = $Hash.Hash.Substring(0, 8).ToLower()
    $StringAsStream.Dispose()
    
    return "$ShortHash-local"
}

# Get project root directory (assuming script is in scripts/ subdirectory)
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$MainPackage = "./cmd/gorepos"
$BinaryName = "gorepos"

# Change to project root
Push-Location $ProjectRoot

try {
    Write-Info "GoRepos Build Script"
    Write-Info "==================="
    
    # Verify Go installation
    try {
        $GoVersion = go version
        Write-Info "Using: $GoVersion"
    }
    catch {
        Write-Error "Go is not installed or not in PATH"
        exit 1
    }

    # Determine version
    if (-not $Version) {
        if ($ContentHash) {
            # Use content-based versioning for local development
            $Version = Get-ContentHash -BaseDir $ProjectRoot
            Write-Info "Using content-based version: $Version"
        } else {
            # Use git-based versioning (default for CI/CD)
            try {
                # Try git describe first (looks for tags)
                $GitVersion = git describe --tags --always --dirty 2>$null
                if ($GitVersion -and $GitVersion -notmatch "^fatal:") {
                    $Version = $GitVersion
                } else {
                    # Fallback: use commit hash + timestamp for uniqueness
                    $GitHash = git rev-parse --short HEAD 2>$null
                    if ($GitHash -and $GitHash -notmatch "^fatal:") {
                        $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
                        $IsDirty = git diff --quiet 2>$null; if ($LASTEXITCODE -ne 0) { "-dirty" } else { "" }
                        $Version = "$GitHash-$Timestamp$IsDirty"
                    } else {
                        # Final fallback: timestamp only
                        $Version = "dev-$(Get-Date -Format 'yyyyMMdd-HHmm')"
                    }
                }
            }
            catch {
                $Version = "dev-$(Get-Date -Format 'yyyyMMdd-HHmm')"
            }
            Write-Info "Using git-based version: $Version"
        }
    }
    Write-Info "Version: $Version"

    # Clean output directory if requested
    if ($Clean) {
        if ($Target -and $Arch) {
            # Selective cleaning - only clean what we're about to build
            $CleanTargets = @()
            if ($Target -eq "all") {
                if ($Arch -eq "all") {
                    $CleanTargets = @("windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64")
                } else {
                    $CleanTargets = @("windows-$Arch", "linux-$Arch", "darwin-$Arch")
                }
            } else {
                if ($Arch -eq "all") {
                    $CleanTargets = @("$Target-amd64", "$Target-arm64")
                } else {
                    $CleanTargets = @("$Target-$Arch")
                }
            }
            
            foreach ($cleanTarget in $CleanTargets) {
                $cleanPath = Join-Path $Output $cleanTarget
                if (Test-Path $cleanPath) {
                    Write-Info "Cleaning $cleanTarget..."
                    Remove-Item -Recurse -Force $cleanPath
                }
            }
        } else {
            # Clean everything if no specific target/arch specified
            if (Test-Path $Output) {
                Write-Info "Cleaning output directory: $Output"
                Remove-Item -Recurse -Force $Output
            }
        }
    }

    # Create output directory
    if (-not (Test-Path $Output)) {
        New-Item -ItemType Directory -Path $Output -Force | Out-Null
    }

    # Run tests if requested
    if ($Test) {
        Write-Info "Running tests..."
        if ($Verbose) {
            go test -v ./...
        } else {
            go test ./...
        }
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Tests failed"
            exit 1
        }
        Write-Success "Tests passed"
    }

    # Define build targets
    $Targets = @()
    
    if ($Target -eq "all") {
        if ($Arch -eq "all" -or -not $Arch) {
            $Targets = @(
                @{ OS = "windows"; Arch = @("amd64", "arm64") },
                @{ OS = "linux"; Arch = @("amd64", "arm64") },
                @{ OS = "darwin"; Arch = @("amd64", "arm64") }
            )
        } else {
            $TargetArch = @($Arch)
            $Targets = @(
                @{ OS = "windows"; Arch = $TargetArch },
                @{ OS = "linux"; Arch = $TargetArch },
                @{ OS = "darwin"; Arch = $TargetArch }
            )
        }
    } elseif ($Target) {
        if ($Arch -eq "all" -or -not $Arch) {
            $TargetArch = @("amd64", "arm64")
        } else {
            $TargetArch = @($Arch)
        }
        $Targets = @(@{ OS = $Target; Arch = $TargetArch })
    } else {
        # Build for current platform
        $CurrentOS = if ($IsWindows) { "windows" } elseif ($IsLinux) { "linux" } elseif ($IsMacOS) { "darwin" } else { "linux" }
        if ($Arch -eq "all" -or -not $Arch) {
            $CurrentArch = @("amd64", "arm64")
        } else {
            $CurrentArch = @($Arch)
        }
        $Targets = @(@{ OS = $CurrentOS; Arch = $CurrentArch })
    }

    # Build for each target
    $BuiltBinaries = @()
    foreach ($TargetInfo in $Targets) {
        $OS = $TargetInfo.OS
        foreach ($Architecture in $TargetInfo.Arch) {
            # Create platform-architecture/toolname-version directory
            $PlatformDir = "${OS}-${Architecture}"
            $VersionDirName = "${BinaryName}-${Version}"
            $VersionDir = Join-Path $Output $PlatformDir $VersionDirName
            
            # Check if version already exists and -Clean was not specified
            if ((Test-Path $VersionDir) -and (-not $Clean)) {
                Write-Warning "Version $Version already exists for ${OS}/${Architecture}. Use -Clean to rebuild or change version."
                continue
            }
            
            if (-not (Test-Path $VersionDir)) {
                New-Item -ItemType Directory -Path $VersionDir -Force | Out-Null
            }
            
            $OutputFile = Join-Path $VersionDir $BinaryName
            if ($OS -eq "windows") {
                $OutputFile += ".exe"
            }
            
            Write-Info "Building for ${OS}/${Architecture}..."
            
            # Set build environment
            $env:GOOS = $OS
            $env:GOARCH = $Architecture
            $env:CGO_ENABLED = "0"
            
            # Build command
            $BuildArgs = @(
                "build",
                "-ldflags", "-s -w -X main.version=$Version",
                "-o", $OutputFile
            )
            
            if ($Verbose) {
                $BuildArgs += "-v"
            }
            
            $BuildArgs += $MainPackage
            
            # Execute build
            & go @BuildArgs
            
            if ($LASTEXITCODE -eq 0) {
                $Size = (Get-Item $OutputFile).Length
                $SizeMB = [math]::Round($Size / 1MB, 2)
                Write-Success "Built ${OS}/${Architecture}: $OutputFile (${SizeMB}MB)"
                
                # Track built binary for listing
                $BinaryFileName = if ($OS -eq "windows") { "$BinaryName.exe" } else { $BinaryName }
                $RelativePath = "${PlatformDir}\${VersionDirName}\${BinaryFileName}"
                $BuiltBinaries += $RelativePath
            } else {
                Write-Error "Build failed for ${OS}/${Architecture}"
                exit 1
            }
        }
    }

    # Create version file
    $VersionFile = Join-Path $Output "VERSION"
    $Version | Out-File -FilePath $VersionFile -Encoding utf8
    Write-Info "Version file created: $VersionFile"

    Write-Success "Build completed successfully!"
    Write-Info "Output directory: $(Resolve-Path $Output)"
    
    # List built binaries
    Write-Info "Built binaries:"
    foreach ($binary in $BuiltBinaries) {
        Write-Host "  - $binary" -ForegroundColor Gray
    }

} catch {
    Write-Error "Build failed: $_"
    exit 1
} finally {
    # Restore environment
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    
    Pop-Location
}