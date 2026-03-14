#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Example of creating a third test environment

.DESCRIPTION
    This demonstrates how easy it is to create additional test environments
    using the shared test_local.ps1 script.
    
    Test Environment 3 (Example):
    - Config: C:\Data\GIT\grtest\example.yaml
    - Base Path: C:\Data\GIT\grtest\example
    - Purpose: Show extensibility of test framework

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
    [switch]$Setup,
    [switch]$SkipHelp,
    [switch]$SkipValidate,
    [switch]$SkipStatus,
    [switch]$SkipGraph,
    [switch]$Groups,
    [switch]$Clone,
    [switch]$Update,
    [string]$SetupIncludes = "",
    [switch]$VerboseOutput
)

# Test environment details - just change these values!
$ConfigFile = "C:\Data\GIT\grtest\example.yaml"
$BasePath = "C:\Data\GIT\grtest\example"
$TestName = "Test Environment 3 (Example Setup)"

# Get script directory and call shared test script
$ScriptDir = Split-Path -Parent $PSScriptRoot
$TestLocalScript = Join-Path $ScriptDir "test_local.ps1"

if (-not (Test-Path $TestLocalScript)) {
    Write-Error "Shared test script not found: $TestLocalScript"
    exit 1
}

# Build arguments for shared script
$TestArgs = @{}
$TestArgs['ConfigFile'] = $ConfigFile
$TestArgs['BasePath'] = $BasePath
$TestArgs['TestName'] = $TestName

if ($Setup) { $TestArgs['Setup'] = $true }
if ($SkipHelp) { $TestArgs['SkipHelp'] = $true }
if ($SkipValidate) { $TestArgs['SkipValidate'] = $true }
if ($SkipStatus) { $TestArgs['SkipStatus'] = $true }
if ($SkipGraph) { $TestArgs['SkipGraph'] = $true }
if ($Groups) { $TestArgs['Groups'] = $true }
if ($Clone) { $TestArgs['Clone'] = $true }
if ($Update) { $TestArgs['Update'] = $true }
if ($VerboseOutput) { $TestArgs['VerboseOutput'] = $true }
if ($SetupIncludes -ne "") { $TestArgs['SetupIncludes'] = $SetupIncludes }

# Execute shared test script
& $TestLocalScript @TestArgs
exit $LASTEXITCODE