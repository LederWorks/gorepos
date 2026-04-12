#!/bin/bash
#
# Build script for GoRepos project (Bash)
#
# This script builds the GoRepos binary with various options for different platforms.
# It handles cross-compilation, versioning, and output organization.
#
# Usage:
#   ./scripts/build.sh [options]
#
# Options:
#   -t, --target PLATFORM     Target platform (windows, linux, darwin, all) [default: all]
#   -a, --arch ARCH           Target architecture (amd64, arm64, all) [default: all]
#   -o, --output DIR          Output directory [default: dist]
#   -v, --version VERSION     Version string to embed
#   -c, --clean               Clean output directory before building
#   --test                    Run tests before building
#   --verbose                 Enable verbose output
#   -h, --help                Show this help message
#
# Examples:
#   ./scripts/build.sh                           # Build for all platforms and architectures
#   ./scripts/build.sh -t all -a amd64 -c       # Build for all platforms with amd64
#   ./scripts/build.sh -t linux --test          # Build for Linux with tests

set -euo pipefail

# Default values
TARGET="all"
ARCH="all"
OUTPUT="dist"
VERSION=""
CONTENT_HASH=false
CLEAN=false
TEST=false
VERBOSE=false

# Color functions
info() {
    echo -e "\033[36mℹ️  $1\033[0m" >&2
}

success() {
    echo -e "\033[32m✅ $1\033[0m" >&2
}

error() {
    echo -e "\033[31m❌ $1\033[0m" >&2
}

warning() {
    echo -e "\033[33m⚠️  $1\033[0m" >&2
}

# Content-based versioning function
get_content_hash() {
    local base_dir="${1:-$(pwd)}"
    
    info "Calculating content hash for Go source files..."
    
    # Find all Go files and relevant config files, excluding vendor
    local content=""
    
    # Process .go files
    while IFS= read -r -d '' file; do
        if [[ "$file" != *"/vendor/"* ]]; then
            local relative_path="${file#"$base_dir"/}"
            local file_content
            file_content=$(cat "$file" 2>/dev/null || echo "")
            content="${content}${relative_path}:${file_content}"
        fi
    done < <(find "$base_dir" -name "*.go" -type f -print0)
    
    # Add go.mod
    if [[ -f "$base_dir/go.mod" ]]; then
        local mod_content
        mod_content=$(cat "$base_dir/go.mod" 2>/dev/null || echo "")
        content="${content}go.mod:${mod_content}"
    fi
    
    # Add go.sum if it exists
    if [[ -f "$base_dir/go.sum" ]]; then
        local sum_content
        sum_content=$(cat "$base_dir/go.sum" 2>/dev/null || echo "")
        content="${content}go.sum:${sum_content}"
    fi
    
    # Create hash and take first 8 characters
    # sha256sum is not available on macOS; fall back to shasum or openssl
    local hash
    if command -v sha256sum &> /dev/null; then
        hash=$(echo -n "$content" | sha256sum | cut -d' ' -f1 | head -c 8)
    elif command -v shasum &> /dev/null; then
        hash=$(echo -n "$content" | shasum -a 256 | cut -d' ' -f1 | head -c 8)
    elif command -v openssl &> /dev/null; then
        hash=$(echo -n "$content" | openssl dgst -sha256 | awk -F'= ' '{print $2}' | head -c 8)
    else
        error "No SHA-256 tool found (sha256sum, shasum, or openssl). Please install one."
        exit 1
    fi
    
    echo "${hash}-local"
}

# Help function
show_help() {
    cat << 'EOF'
GoRepos Build Script

Usage:
  ./scripts/build.sh [options]

Options:
  -t, --target PLATFORM     Target platform (windows, linux, darwin, all) [default: all]
  -a, --arch ARCH           Target architecture (amd64, arm64, all) [default: all]
  -o, --output DIR          Output directory [default: dist]
  -v, --version VERSION     Version string to embed
  --content-hash            Use content-based versioning for local development
  -c, --clean               Clean output directory before building
  --test                    Run tests before building
  --verbose                 Enable verbose output
  -h, --help                Show this help message

Examples:
  ./scripts/build.sh                           # Build for all platforms and architectures
  ./scripts/build.sh -t all -a amd64 -c       # Build for all platforms with amd64
  ./scripts/build.sh -t linux --test          # Build for Linux with tests
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--target)
            TARGET="$2"
            shift 2
            ;;
        -a|--arch)
            ARCH="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        --content-hash)
            CONTENT_HASH=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        --test)
            TEST=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Get project root directory (assuming script is in scripts/ subdirectory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MAIN_PACKAGE="./cmd/gorepos"
BINARY_NAME="gorepos"

# Change to project root
cd "$PROJECT_ROOT"

info "GoRepos Build Script"
info "==================="

# Verify Go installation
if ! command -v go &> /dev/null; then
    error "Go is not installed or not in PATH"
    exit 1
fi

GO_VERSION=$(go version)
info "Using: $GO_VERSION"

# Determine version
if [[ -z "$VERSION" ]]; then
    if [[ "$CONTENT_HASH" == true ]]; then
        # Use content-based versioning for local development
        VERSION=$(get_content_hash "$(pwd)")
        info "Using content-based version: $VERSION"
    else
        # Use git-based versioning (default for CI/CD)
        if command -v git &> /dev/null && git rev-parse --git-dir > /dev/null 2>&1; then
            # Try git describe first (looks for tags)
            VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "")
            if [[ -z "$VERSION" || "$VERSION" == *"fatal:"* ]]; then
                # Fallback: use commit hash + timestamp for uniqueness
                git_hash=$(git rev-parse --short HEAD 2>/dev/null || echo "")
                if [[ -n "$git_hash" && "$git_hash" != *"fatal:"* ]]; then
                    timestamp=$(date +%Y%m%d-%H%M)
                    is_dirty=""
                    if ! git diff --quiet 2>/dev/null; then
                        is_dirty="-dirty"
                    fi
                    VERSION="$git_hash-$timestamp$is_dirty"
                else
                    # Final fallback: timestamp only
                    VERSION="dev-$(date +%Y%m%d-%H%M)"
                fi
            fi
        else
            VERSION="dev-$(date +%Y%m%d-%H%M)"
        fi
        info "Using git-based version: $VERSION"
    fi
fi

# Clean output directory if requested
if [[ "$CLEAN" == true ]]; then
    if [[ -n "$TARGET" && -n "$ARCH" ]]; then
        # Selective cleaning - only clean what we're about to build
        declare -a CLEAN_TARGETS=()
        if [[ "$TARGET" == "all" ]]; then
            if [[ "$ARCH" == "all" ]]; then
                CLEAN_TARGETS=("windows-amd64" "windows-arm64" "linux-amd64" "linux-arm64" "darwin-amd64" "darwin-arm64")
            else
                CLEAN_TARGETS=("windows-$ARCH" "linux-$ARCH" "darwin-$ARCH")
            fi
        else
            if [[ "$ARCH" == "all" ]]; then
                CLEAN_TARGETS=("$TARGET-amd64" "$TARGET-arm64")
            else
                CLEAN_TARGETS=("$TARGET-$ARCH")
            fi
        fi
        
        for clean_target in "${CLEAN_TARGETS[@]}"; do
            clean_path="$OUTPUT/$clean_target"
            if [[ -d "$clean_path" ]]; then
                info "Cleaning $clean_target..."
                rm -rf "$clean_path"
            fi
        done
    else
        # Clean everything if no specific target/arch specified
        if [[ -d "$OUTPUT" ]]; then
            info "Cleaning output directory: $OUTPUT"
            rm -rf "$OUTPUT"
        fi
    fi
fi

# Create output directory
mkdir -p "$OUTPUT"

# Run tests if requested
if [[ "$TEST" == true ]]; then
    info "Running tests..."
    if [[ "$VERBOSE" == true ]]; then
        go test -v ./...
    else
        go test ./...
    fi
    success "Tests passed"
fi

# Define build targets
declare -a BUILD_TARGETS=()

if [[ "$TARGET" == "all" ]]; then
    if [[ "$ARCH" == "all" || -z "$ARCH" ]]; then
        BUILD_TARGETS=(
            "windows:amd64" "windows:arm64"
            "linux:amd64" "linux:arm64"
            "darwin:amd64" "darwin:arm64"
        )
    else
        BUILD_TARGETS=(
            "windows:$ARCH"
            "linux:$ARCH"
            "darwin:$ARCH"
        )
    fi
elif [[ -n "$TARGET" ]]; then
    if [[ "$ARCH" == "all" || -z "$ARCH" ]]; then
        BUILD_TARGETS=("$TARGET:amd64" "$TARGET:arm64")
    else
        BUILD_TARGETS=("$TARGET:$ARCH")
    fi
else
    # Detect current platform
    case "$(uname -s)" in
        Linux*)     CURRENT_OS=linux;;
        Darwin*)    CURRENT_OS=darwin;;
        CYGWIN*|MINGW*|MSYS*) CURRENT_OS=windows;;
        *)          CURRENT_OS=linux;;
    esac
    if [[ "$ARCH" == "all" || -z "$ARCH" ]]; then
        BUILD_TARGETS=("$CURRENT_OS:amd64" "$CURRENT_OS:arm64")
    else
        BUILD_TARGETS=("$CURRENT_OS:$ARCH")
    fi
fi

# Build for each target
declare -a BUILT_BINARIES=()
for target in "${BUILD_TARGETS[@]}"; do
    IFS=':' read -r os arch <<< "$target"
    
    # Create platform-architecture/toolname-version directory
    platform_dir="${os}-${arch}"
    version_dir_name="${BINARY_NAME}-${VERSION}"
    version_dir="$OUTPUT/$platform_dir/$version_dir_name"
    
    # Check if version already exists and -Clean was not specified
    if [[ -d "$version_dir" && "$CLEAN" != true ]]; then
        warning "Version $VERSION already exists for ${os}/${arch}. Use --clean to rebuild or change version."
        continue
    fi
    
    mkdir -p "$version_dir"
    
    output_file="$version_dir/$BINARY_NAME"
    if [[ "$os" == "windows" ]]; then
        output_file="${output_file}.exe"
    fi
    
    info "Building for ${os}/${arch}..."
    
    # Set build environment
    export GOOS="$os"
    export GOARCH="$arch"
    export CGO_ENABLED=0
    
    # Build command arguments
    build_args=(
        "build"
        "-ldflags" "-s -w -X github.com/LederWorks/gorepos/cmd/gorepos.version=$VERSION"
        "-o" "$output_file"
    )
    
    if [[ "$VERBOSE" == true ]]; then
        build_args+=("-v")
    fi
    
    build_args+=("$MAIN_PACKAGE")
    
    # Execute build
    if [[ "$VERBOSE" == true ]]; then
        echo "Debug: Full command would be:"
        echo "  go" "${build_args[*]}"
    fi
    
    if go "${build_args[@]}"; then
        size=$(wc -c < "$output_file" | tr -d ' ')
        size=$(( size / 1024 ))
        size="${size}K"
        success "Built ${os}/${arch}: $output_file ($size)"
        
        # Track built binary for listing
        relative_path=${output_file#"$OUTPUT"/}
        BUILT_BINARIES+=("$relative_path")
    else
        error "Build failed for ${os}/${arch}"
        exit 1
    fi
done

# Create version file
echo "$VERSION" > "$OUTPUT/VERSION"
info "Version file created: $OUTPUT/VERSION"

success "Build completed successfully!"
info "Output directory: $(cd "$OUTPUT" && pwd)"

# List built binaries
info "Built binaries:"
for binary in "${BUILT_BINARIES[@]}"; do
    echo "  - $binary"
done

# Cleanup environment variables
unset GOOS GOARCH CGO_ENABLED