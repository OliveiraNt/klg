#!/usr/bin/env sh
# klg installer
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/OliveiraNt/klg/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/OliveiraNt/klg/main/install.sh | sh -s -- -b /usr/local/bin
#   curl -fsSL https://raw.githubusercontent.com/OliveiraNt/klg/main/install.sh | sh -s -- v0.1.0
#
# Opções:
#   -b <dir>   Diretório de instalação (padrão: /usr/local/bin ou $HOME/.local/bin)
#   -v         Modo verboso
#   <version>  Versão a instalar (padrão: latest)

set -e

OWNER="OliveiraNt"
REPO="klg"
BIN_NAME="klg"

INSTALL_DIR=""
VERBOSE=0
VERSION=""

log() { printf '%s\n' "$*" >&2; }
info() { log "==> $*"; }
warn() { log "WARN: $*"; }
err() { log "ERRO: $*"; exit 1; }

usage() {
    cat <<EOF
Instala o binário $BIN_NAME a partir dos releases do GitHub.

Uso: install.sh [-b <dir>] [-v] [<version>]

  -b <dir>   diretório de instalação
  -v         modo verboso
  <version>  versão (ex.: v0.1.0). Padrão: latest
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        -b) INSTALL_DIR="$2"; shift 2 ;;
        -v) VERBOSE=1; shift ;;
        -h|--help) usage; exit 0 ;;
        -*) err "flag desconhecida: $1" ;;
        *) VERSION="$1"; shift ;;
    esac
done

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux) echo "Linux" ;;
        Darwin) echo "Darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "Windows" ;;
        *) err "SO não suportado: $os" ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) echo "x86_64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) err "arquitetura não suportada: $arch" ;;
    esac
}

need() {
    command -v "$1" >/dev/null 2>&1 || err "comando requerido: $1"
}

http_get() {
    url="$1"; out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$out"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$out" "$url"
    else
        err "curl ou wget é necessário"
    fi
}

resolve_latest() {
    api="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
    tmp="$(mktemp)"
    http_get "$api" "$tmp"
    # extrai tag_name sem depender de jq
    tag="$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$tmp" | head -n1)"
    rm -f "$tmp"
    [ -n "$tag" ] || err "não foi possível resolver a versão mais recente"
    echo "$tag"
}

pick_install_dir() {
    if [ -n "$INSTALL_DIR" ]; then
        echo "$INSTALL_DIR"; return
    fi
    if [ -w "/usr/local/bin" ] 2>/dev/null; then
        echo "/usr/local/bin"
    else
        echo "$HOME/.local/bin"
    fi
}

main() {
    OS="$(detect_os)"
    ARCH="$(detect_arch)"

    if [ "$OS" = "Windows" ] && [ "$ARCH" = "arm64" ]; then
        err "Windows arm64 não é distribuído; use go install"
    fi

    if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
        VERSION="$(resolve_latest)"
    fi
    # normaliza sem v
    VERSION_NUM="${VERSION#v}"

    if [ "$OS" = "Windows" ]; then
        EXT="zip"
        BIN="${BIN_NAME}.exe"
    else
        EXT="tar.gz"
        BIN="${BIN_NAME}"
    fi

    ARCHIVE="${BIN_NAME}_${VERSION_NUM}_${OS}_${ARCH}.${EXT}"
    URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
    SUMS_URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/checksums.txt"

    info "SO: $OS  arch: $ARCH  versão: $VERSION"
    [ "$VERBOSE" = "1" ] && info "URL: $URL"

    TMP="$(mktemp -d)"
    trap 'rm -rf "$TMP"' EXIT

    info "baixando $ARCHIVE"
    http_get "$URL" "$TMP/$ARCHIVE"

    if http_get "$SUMS_URL" "$TMP/checksums.txt" 2>/dev/null; then
        if command -v sha256sum >/dev/null 2>&1; then
            (cd "$TMP" && grep " $ARCHIVE\$" checksums.txt | sha256sum -c -) \
                || err "checksum inválido"
            info "checksum OK"
        elif command -v shasum >/dev/null 2>&1; then
            (cd "$TMP" && grep " $ARCHIVE\$" checksums.txt | shasum -a 256 -c -) \
                || err "checksum inválido"
            info "checksum OK"
        else
            warn "sha256sum/shasum ausente; pulando verificação"
        fi
    else
        warn "checksums.txt não encontrado; pulando verificação"
    fi

    info "extraindo"
    case "$EXT" in
        tar.gz) tar -xzf "$TMP/$ARCHIVE" -C "$TMP" ;;
        zip)    need unzip; unzip -q "$TMP/$ARCHIVE" -d "$TMP" ;;
    esac

    [ -f "$TMP/$BIN" ] || err "binário $BIN não encontrado no arquivo"

    DEST="$(pick_install_dir)"
    mkdir -p "$DEST"

    if [ -w "$DEST" ]; then
        install -m 0755 "$TMP/$BIN" "$DEST/$BIN"
    else
        info "$DEST não é gravável; tentando via sudo"
        need sudo
        sudo install -m 0755 "$TMP/$BIN" "$DEST/$BIN"
    fi

    info "instalado em $DEST/$BIN"
    case ":$PATH:" in
        *":$DEST:"*) ;;
        *) warn "$DEST não está no PATH; adicione com: export PATH=\"$DEST:\$PATH\"" ;;
    esac

    "$DEST/$BIN" --help >/dev/null 2>&1 || true
    info "pronto. Execute: $BIN_NAME --help"
}

main
