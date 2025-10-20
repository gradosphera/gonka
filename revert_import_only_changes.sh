#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
revert_import_only_changes.sh [--apply|--dry-run]

Scans modified .go files and compares the current working copy to HEAD with both
versions normalized by `goimports`. If the normalized files are identical, the
change is deemed "imports/whitespace-only".

--dry-run  (default)  Print which files WOULD be reverted.
--apply               Actually revert those files to HEAD.

Notes:
- Only considers unstaged changes (like `git diff --name-only`).
- Skips files not present in HEAD (e.g., brand-new files).
USAGE
}

MODE="dry-run"
if [[ "${1:-}" == "--apply" ]]; then
  MODE="apply"
elif [[ "${1:-}" == "--dry-run" || -z "${1:-}" ]]; then
  MODE="dry-run"
else
  usage
  exit 1
fi

# Ensure goimports exists
if ! command -v goimports >/dev/null 2>&1; then
  echo "Error: goimports not found in PATH." >&2
  exit 2
fi

# Get unstaged changed files (handle spaces/newlines safely)
# Use -z to NUL-terminate entries.
changed_files=()
while IFS= read -r -d '' file; do
  changed_files+=("$file")
done < <(git diff --name-only -z)

# Nothing to do?
if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "No unstaged changes."
  exit 0
fi

# Process each changed file
for file in "${changed_files[@]}"; do
  # Skip non-go files
  if [[ "${file}" != *.go ]]; then
    continue
  fi

  # Skip if file no longer exists in the working tree (e.g., deleted)
  if [[ ! -e "${file}" ]]; then
    continue
  fi

  # Skip files that didn't exist in HEAD (new files)
  if ! git cat-file -e "HEAD:${file}" 2>/dev/null; then
    # New file; can't compare to HEAD. Treat as having real changes.
    echo "Would NOT revert ${file} (new file or not in HEAD)"
    continue
  fi

  tmp_original="$(mktemp)"
  tmp_current="$(mktemp)"
  cleanup() { rm -f "${tmp_original}" "${tmp_current}"; }
  trap cleanup EXIT

  # Normalize both versions through goimports
  if ! git show "HEAD:${file}" | goimports > "${tmp_original}"; then
    echo "Warning: failed to normalize HEAD version for ${file}" >&2
    cleanup; trap - EXIT; exit 3
  fi
  if ! goimports < "${file}" > "${tmp_current}"; then
    echo "Warning: failed to normalize working copy for ${file}" >&2
    cleanup; trap - EXIT; exit 3
  fi

  if diff -q "${tmp_original}" "${tmp_current}" >/dev/null; then
    # Only imports/whitespace changed
    if [[ "${MODE}" == "apply" ]]; then
      # Revert working copy to HEAD
      # (use git restore if available; fallback to checkout)
      if git restore --source=HEAD --worktree -- "${file}" 2>/dev/null; then
        echo "Reverted ${file} (imports/whitespace-only)"
      else
        git checkout -- "${file}"
        echo "Reverted ${file} (imports/whitespace-only)"
      fi
    else
      echo "Would revert ${file} (imports/whitespace-only)"
    fi
  else
    echo "Would NOT revert ${file} (has non-import/whitespace changes)"
  fi

  cleanup
  trap - EXIT
done