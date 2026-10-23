#!/usr/bin/env sh
set -eu

msg_file=${1:?commit message file required}
header=$(sed -n '1p' "$msg_file" | tr -d '\r')

if [ -z "$header" ]; then
  echo "commit message header is required" >&2
  exit 1
fi

case "$header" in
  Merge\ *|Revert\ *)
    exit 0
    ;;
esac

if [ "${#header}" -gt 100 ]; then
  echo "commit message header must be 100 characters or fewer" >&2
  exit 1
fi

pattern='^(add|build|chore|ci|docs|feat|fix|init|perf|refactor|revert|style|test)(\([a-z0-9._/-]+\))?!?: [^ ].+$'

if ! printf '%s\n' "$header" | grep -Eq "$pattern"; then
  echo "commit message must match Conventional Commits: type(scope): subject" >&2
  exit 1
fi

if ! awk 'NR > 1 && NF && $0 !~ /^Signed-off-by: .+ <[^>]+>$/ {bad=1} END {exit bad}' "$msg_file"; then
  echo "commit message body may contain only Signed-off-by trailers" >&2
  exit 1
fi
