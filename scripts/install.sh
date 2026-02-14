#!/bin/sh
set -eu

main() {
    REPO="708u/cctidy"
    BINARY="cctidy"
    GITHUB_BASE="https://github.com/${REPO}"
    INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

    detect_platform
    resolve_version

    printf "Installing %s v%s (%s/%s)...\n" \
        "${BINARY}" "${VERSION}" "${OS}" "${ARCH}"

    download_and_verify
    install_binary

    printf "Successfully installed %s to %s/%s\n" \
        "${BINARY}" "${INSTALL_DIR}" "${BINARY}"
}

detect_platform() {
    OS=$(uname -s)
    ARCH=$(uname -m)

    case "${OS}" in
        Darwin) OS="Darwin" ;;
        Linux)  OS="Linux" ;;
        *)
            printf "error: unsupported OS: %s\n" "${OS}" >&2
            printf "Use install.ps1 for Windows\n" >&2
            exit 1
            ;;
    esac

    case "${ARCH}" in
        x86_64 | amd64)  ARCH="x86_64" ;;
        aarch64 | arm64) ARCH="arm64" ;;
        i386 | i686)     ARCH="i386" ;;
        *)
            printf "error: unsupported architecture: %s\n" \
                "${ARCH}" >&2
            exit 1
            ;;
    esac
}

resolve_version() {
    if [ -n "${VERSION:-}" ]; then
        VERSION=$(printf '%s' "${VERSION}" | sed 's/^v//')
        return
    fi

    REDIRECT=""
    if command -v curl >/dev/null 2>&1; then
        REDIRECT=$(curl -sSf -o /dev/null \
            -w '%{redirect_url}' \
            "${GITHUB_BASE}/releases/latest" \
            2>/dev/null) || true
    elif command -v wget >/dev/null 2>&1; then
        REDIRECT=$(wget --spider -S \
            "${GITHUB_BASE}/releases/latest" 2>&1 |
            grep -i 'Location:' | tail -1 |
            awk '{print $2}' | tr -d '\r') || true
    else
        printf "error: curl or wget is required\n" >&2
        exit 1
    fi

    VERSION=$(printf '%s' "${REDIRECT}" |
        grep -o '[^/]*$' | sed 's/^v//')

    if [ -z "${VERSION}" ]; then
        printf "error: failed to detect latest version\n" >&2
        exit 1
    fi
}

download() {
    dl_url="$1"
    dl_dest="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -sSfL -o "${dl_dest}" "${dl_url}"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "${dl_dest}" "${dl_url}"
    else
        printf "error: curl or wget is required\n" >&2
        exit 1
    fi
}

verify_checksum() {
    cs_dir="$1"
    cs_target="$2"
    cs_file="$3"

    expected=$(grep -F "${cs_target}" \
        "${cs_dir}/${cs_file}" | awk '{print $1}')

    if [ -z "${expected}" ]; then
        printf "error: checksum not found for %s\n" \
            "${cs_target}" >&2
        exit 1
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "${cs_dir}/${cs_target}" |
            awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 \
            "${cs_dir}/${cs_target}" | awk '{print $1}')
    else
        printf "warning: sha256 tool not found, " >&2
        printf "skipping verification\n" >&2
        return
    fi

    if [ "${expected}" != "${actual}" ]; then
        printf "error: checksum mismatch\n" >&2
        printf "  expected: %s\n" "${expected}" >&2
        printf "  actual:   %s\n" "${actual}" >&2
        exit 1
    fi
}

download_and_verify() {
    ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
    CHECKSUMS="${BINARY}_${VERSION}_checksums.txt"
    DL_BASE="${GITHUB_BASE}/releases/download/v${VERSION}"

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "${TMP_DIR}"' EXIT

    download "${DL_BASE}/${ARCHIVE}" \
        "${TMP_DIR}/${ARCHIVE}"
    download "${DL_BASE}/${CHECKSUMS}" \
        "${TMP_DIR}/${CHECKSUMS}"

    verify_checksum "${TMP_DIR}" "${ARCHIVE}" \
        "${CHECKSUMS}"
}

install_binary() {
    tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

    BIN_PATH=""
    for f in "${TMP_DIR}/${BINARY}" \
             "${TMP_DIR}"/*/"${BINARY}"; do
        if [ -f "$f" ]; then
            BIN_PATH="$f"
            break
        fi
    done

    if [ -z "${BIN_PATH}" ]; then
        printf "error: binary not found in archive\n" >&2
        exit 1
    fi

    mkdir -p "${INSTALL_DIR}"
    install -m 755 "${BIN_PATH}" \
        "${INSTALL_DIR}/${BINARY}"
}

main "$@"
