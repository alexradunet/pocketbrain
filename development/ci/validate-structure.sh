#!/usr/bin/env bash
set -euo pipefail

BASE_REF="${1:-origin/main}"

if [ "$BASE_REF" = "--working-tree" ]; then
  mapfile -t ADDED_PATHS < <(
    {
      git diff --name-status --cached | awk '$1=="A"{print $2} $1 ~ /^R[0-9]*$/{print $3}'
      git ls-files --others --exclude-standard
    } | sort -u
  )
else
  if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
    echo "Unable to resolve base ref: $BASE_REF"
    exit 1
  fi
  mapfile -t ADDED_PATHS < <(git diff --name-status "$BASE_REF"...HEAD | awk '$1=="A"{print $2} $1 ~ /^R[0-9]*$/{print $3}')
fi

if [ "${#ADDED_PATHS[@]}" -eq 0 ]; then
  echo "No added files to validate."
  exit 0
fi

FAILURES=()

for path in "${ADDED_PATHS[@]}"; do
  case "$path" in
    *.md)
      if [[ "$path" != docs/* ]] && [[ "$path" != "README.md" ]] && [[ "$path" != */README.md ]]; then
        FAILURES+=("Markdown file outside docs contract: $path")
      fi
      ;;
    *.sh)
      if [[ "$path" != development/* ]] && [[ "$path" != scripts/* ]] && [[ "$path" != pocketbrain/scripts/* ]]; then
        FAILURES+=("Shell script outside development/scripts contract: $path")
      fi
      if [[ "$path" == scripts/*.sh ]] && [[ "$path" != scripts/*/*.sh ]]; then
        case "$path" in
          scripts/install-debian.sh|scripts/docker-build.sh|scripts/docker-logs.sh|scripts/docker-shell.sh|scripts/docker-entrypoint.sh)
            ;;
          *)
            FAILURES+=("New root scripts/*.sh are disallowed. Use scripts/ops or scripts/runtime: $path")
            ;;
        esac
      fi
      ;;
  esac
done

if [ "${#FAILURES[@]}" -gt 0 ]; then
  printf '%s\n' "Structure contract violations:" >&2
  printf ' - %s\n' "${FAILURES[@]}" >&2
  exit 1
fi

echo "Structure contract check passed."
