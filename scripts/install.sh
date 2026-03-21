#!/bin/bash
#
# DevContext AI Installer
# Usage: curl -sSL https://get.devctx.sh | sh
#

set -e

REPO="devctx/devctx"
INSTALL_DIR="${DEVCTX_INSTALL_DIR:-$HOME/.devctx/bin}"
CONFIG_DIR="$HOME/.devctx"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_banner() {
    echo -e "${BLUE}"
    echo "  ____             ____            _            _   "
    echo " |  _ \  _____   _/ ___|___  _ __ | |_ _____  _| |_ "
    echo " | | | |/ _ \ \ / / |   / _ \\| '_ \\| __/ _ \\ \\/ / __|"
    echo " | |_| |  __/\ V /| |__| (_) | | | | ||  __/>  <| |_ "
    echo " |____/ \___| \_/  \____\___/|_| |_|\__\___/_/\_\\\\__|"
    echo -e "${NC}"
    echo "  AI-powered codebase intelligence"
    echo ""
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

detect_os() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)  OS="linux" ;;
        Darwin*) OS="darwin" ;;
        *)       error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64)   ARCH="arm64" ;;
        *)       error "Unsupported architecture: $ARCH" ;;
    esac

    info "Detected: $OS/$ARCH"
}

get_latest_version() {
    info "Fetching latest version..."
    VERSION=$(curl -sS "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        VERSION="v0.1.0"
        warning "Could not fetch latest version, using $VERSION"
    fi

    info "Version: $VERSION"
}

download_binary() {
    BINARY_NAME="devctx_${VERSION#v}_${OS}_${ARCH}"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY_NAME}.tar.gz"

    info "Downloading from: $DOWNLOAD_URL"

    mkdir -p "$INSTALL_DIR"

    # Download and extract
    if command -v curl &> /dev/null; then
        curl -sSL "$DOWNLOAD_URL" | tar xz -C "$INSTALL_DIR"
    elif command -v wget &> /dev/null; then
        wget -qO- "$DOWNLOAD_URL" | tar xz -C "$INSTALL_DIR"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi

    # Make executable
    chmod +x "$INSTALL_DIR/devctx"

    success "Binary installed to $INSTALL_DIR/devctx"
}

download_daemon() {
    DAEMON_URL="https://github.com/$REPO/releases/download/$VERSION/daemon.jar"

    info "Downloading daemon..."

    mkdir -p "$INSTALL_DIR"

    if command -v curl &> /dev/null; then
        curl -sSL -o "$INSTALL_DIR/daemon.jar" "$DAEMON_URL"
    elif command -v wget &> /dev/null; then
        wget -q -O "$INSTALL_DIR/daemon.jar" "$DAEMON_URL"
    fi

    success "Daemon installed to $INSTALL_DIR/daemon.jar"
}

setup_directories() {
    info "Setting up directories..."

    mkdir -p "$CONFIG_DIR"
    mkdir -p "$CONFIG_DIR/index"
    mkdir -p "$CONFIG_DIR/repos"
    mkdir -p "$CONFIG_DIR/logs"

    success "Directories created"
}

setup_path() {
    # Detect shell
    SHELL_NAME=$(basename "$SHELL")

    case "$SHELL_NAME" in
        bash) RC_FILE="$HOME/.bashrc" ;;
        zsh)  RC_FILE="$HOME/.zshrc" ;;
        fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
        *)    RC_FILE="$HOME/.profile" ;;
    esac

    # Check if already in PATH
    if [[ ":$PATH:" == *":$INSTALL_DIR:"* ]]; then
        info "PATH already configured"
        return
    fi

    info "Adding to PATH in $RC_FILE"

    if [ "$SHELL_NAME" = "fish" ]; then
        echo "set -gx PATH \"$INSTALL_DIR\" \$PATH" >> "$RC_FILE"
    else
        echo "" >> "$RC_FILE"
        echo "# DevContext AI" >> "$RC_FILE"
        echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$RC_FILE"
    fi

    success "PATH updated. Run 'source $RC_FILE' or restart your terminal."
}

check_dependencies() {
    # Check for Java (required for daemon)
    if ! command -v java &> /dev/null; then
        warning "Java not found. The daemon requires Java 21+."
        warning "Install with: brew install openjdk@21 (macOS) or apt install openjdk-21-jdk (Ubuntu)"
    else
        JAVA_VERSION=$(java -version 2>&1 | head -1 | cut -d'"' -f2 | cut -d'.' -f1)
        if [ "$JAVA_VERSION" -lt 21 ] 2>/dev/null; then
            warning "Java $JAVA_VERSION found, but Java 21+ is recommended."
        else
            success "Java $JAVA_VERSION found"
        fi
    fi
}

verify_installation() {
    info "Verifying installation..."

    if [ -x "$INSTALL_DIR/devctx" ]; then
        VERSION_OUTPUT=$("$INSTALL_DIR/devctx" --help 2>&1 | head -1)
        success "DevContext installed successfully!"
        echo ""
        echo "  Run 'devctx init' to get started"
        echo ""
    else
        error "Installation verification failed"
    fi
}

main() {
    print_banner

    detect_os
    get_latest_version
    setup_directories
    download_binary
    download_daemon
    setup_path
    check_dependencies
    verify_installation

    echo ""
    echo -e "${GREEN}Installation complete!${NC}"
    echo ""
    echo "Quick start:"
    echo "  1. Restart your terminal or run: source ~/.zshrc"
    echo "  2. Run: devctx init"
    echo "  3. Run: devctx ask \"explain main.go\""
    echo ""
}

main "$@"
