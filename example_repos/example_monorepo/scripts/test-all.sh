#!/usr/bin/env bash
# Run every project's test suite.
set -euo pipefail

root="$(git rev-parse --show-toplevel)"

for project in python_lib services/api; do
    echo "==> ${project}"
    (cd "${root}/${project}" && uv run --locked pytest -q)
done

echo "==> go_lib"
(cd "${root}/go_lib" && go test ./...)
