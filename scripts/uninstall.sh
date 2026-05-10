#!/bin/sh
# wtguard one-line uninstaller.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cuongtranba/wtguard/main/scripts/uninstall.sh | sh
# Env override:
#   WTGUARD_DIR  install root (default: $HOME/.wtguard)
#
# Note: GitHub branch protection applied via `wtguard install --remote-protect`
# is NOT removed automatically (it affects shared repo state). Remove it via
# the GitHub UI or `gh api -X DELETE repos/{owner}/{repo}/branches/main/protection`.
set -eu

DIR="${WTGUARD_DIR:-$HOME/.wtguard}"

# 1. best-effort: undo per-repo hook in current repo --------------------------
if [ -x "$DIR/bin/wtguard" ] && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  "$DIR/bin/wtguard" uninstall >/dev/null 2>&1 || true
fi

# 2. unset global init.templateDir if it points at us ------------------------
current_template=$(git config --global --get init.templateDir 2>/dev/null || true)
if [ "$current_template" = "$DIR/template" ]; then
  git config --global --unset init.templateDir
  echo "wtguard: cleared global init.templateDir"
fi

# 3. strip marker block from shell rc (idempotent) ----------------------------
unpatch_rc() {
  rc="$1"
  [ -f "$rc" ] || return 0
  if ! grep -q '^# >>> wtguard >>>' "$rc" 2>/dev/null; then
    return 0
  fi
  awk '
    BEGIN { skip = 0 }
    /^# >>> wtguard >>>/ { skip = 1; next }
    /^# <<< wtguard <<<$/ { skip = 0; next }
    !skip { print }
  ' "$rc" > "$rc.wtguard.tmp" && mv "$rc.wtguard.tmp" "$rc"
  echo "wtguard: cleaned $rc"
}

unpatch_rc "$HOME/.zshrc"
unpatch_rc "$HOME/.bashrc"
unpatch_rc "$HOME/.bash_profile"
unpatch_rc "$HOME/.profile"

# 4. remove install dir -------------------------------------------------------
if [ -d "$DIR" ]; then
  rm -rf "$DIR"
  echo "wtguard: removed $DIR"
fi

echo
echo "wtguard uninstalled."
echo "open a new shell to drop the old PATH from your environment."
