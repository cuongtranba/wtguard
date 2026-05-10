#!/bin/sh
# wtguard one-line installer.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cuongtranba/wtguard/main/scripts/install.sh | sh
# Env overrides:
#   WTGUARD_DIR     install root           (default: $HOME/.wtguard)
#   WTGUARD_VERSION specific tag like v1.2.3 (default: latest GitHub release)
set -eu

OWNER=cuongtranba
REPO=wtguard
DIR="${WTGUARD_DIR:-$HOME/.wtguard}"
BIN_DIR="$DIR/bin"
TMPL_DIR="$DIR/template/hooks"

mkdir -p "$BIN_DIR" "$TMPL_DIR"

# 1. detect OS and arch ------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  darwin|linux) ;;
  *) echo "wtguard: unsupported OS '$os'" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64)  goreleaser_arch=x86_64 ;;
  arm64|aarch64) goreleaser_arch=arm64 ;;
  *) echo "wtguard: unsupported arch '$arch'" >&2; exit 1 ;;
esac

# GoReleaser archive uses Title-cased OS (Darwin / Linux)
os_title=$(printf '%s' "$os" | awk '{print toupper(substr($0,1,1)) substr($0,2)}')

# 2. resolve version ----------------------------------------------------------
version="${WTGUARD_VERSION:-}"
if [ -z "$version" ]; then
  version=$(
    curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\(v[^"]*\)".*/\1/p' \
      | head -1
  ) || true
fi

# 3. download release binary, fall back to `go install` -----------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

downloaded=0
if [ -n "$version" ]; then
  archive="wtguard_${version#v}_${os_title}_${goreleaser_arch}.tar.gz"
  url="https://github.com/$OWNER/$REPO/releases/download/$version/$archive"
  echo "wtguard: trying $url"
  if curl -fsSL -o "$tmp/wtguard.tar.gz" "$url" 2>/dev/null; then
    if tar -xzf "$tmp/wtguard.tar.gz" -C "$tmp" wtguard 2>/dev/null; then
      install -m 0755 "$tmp/wtguard" "$BIN_DIR/wtguard"
      downloaded=1
    fi
  fi
fi

if [ "$downloaded" -eq 0 ]; then
  if command -v go >/dev/null 2>&1; then
    echo "wtguard: no release binary, building from source via 'go install'"
    GOBIN="$BIN_DIR" go install "github.com/$OWNER/$REPO/cmd/wtguard@latest"
  else
    echo "wtguard: no release binary for $os_title/$goreleaser_arch and 'go' is not installed" >&2
    echo "         install Go (https://go.dev/dl/) and re-run, or grab a binary from" >&2
    echo "         https://github.com/$OWNER/$REPO/releases" >&2
    exit 1
  fi
fi

# 4. symlink git wrapper (proxy layer) ----------------------------------------
ln -sf "$BIN_DIR/wtguard" "$BIN_DIR/git"

# 5. hook shim in init template (auto-installs hook in every new clone) ------
cat > "$TMPL_DIR/pre-commit" <<'HOOK'
#!/bin/sh
# wtguard managed - do not edit
exec "${WTGUARD_BIN:-wtguard}" hook pre-commit "$@"
HOOK
chmod 0755 "$TMPL_DIR/pre-commit"

current_template=$(git config --global --get init.templateDir 2>/dev/null || true)
if [ -z "$current_template" ] || [ "$current_template" = "$DIR/template" ]; then
  git config --global init.templateDir "$DIR/template"
else
  echo "wtguard: leaving existing init.templateDir='$current_template' alone"
  echo "         set it to '$DIR/template' manually for the auto-install belt"
fi

# 6. patch shell rc (marker-guarded, idempotent) ------------------------------
patch_rc() {
  rc="$1"
  [ -f "$rc" ] || return 0
  if grep -q '^# >>> wtguard >>>' "$rc" 2>/dev/null; then
    return 0
  fi
  {
    printf '\n'
    printf '# >>> wtguard >>>\n'
    printf 'export PATH="%s:$PATH"\n' "$BIN_DIR"
    printf '# <<< wtguard <<<\n'
  } >> "$rc"
  echo "wtguard: patched $rc"
}

case "${SHELL:-}" in
  *zsh*)
    patch_rc "$HOME/.zshrc"
    ;;
  *bash*)
    patch_rc "$HOME/.bashrc"
    patch_rc "$HOME/.bash_profile"
    ;;
  *)
    patch_rc "$HOME/.profile"
    ;;
esac

# 7. best-effort: install hook in current repo if we are inside one ----------
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  "$BIN_DIR/wtguard" install >/dev/null 2>&1 || true
fi

# 8. report -------------------------------------------------------------------
echo
echo "wtguard installed at $DIR"
echo "  binary:   $BIN_DIR/wtguard"
echo "  proxy:    $BIN_DIR/git -> wtguard"
echo "  template: $DIR/template (init.templateDir)"
echo
echo "open a new shell, or run:  source ~/.zshrc   (or your rc file)"
echo "version: $("$BIN_DIR/wtguard" --version 2>&1 | head -1)"
