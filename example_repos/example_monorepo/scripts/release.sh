#!/usr/bin/env bash
# Build the Python packages and tag a release: scripts/release.sh 1.2.3
set -euo pipefail

version="${1:?usage: $0 <version>}"

case "${version}" in
    [0-9]*.[0-9]*.[0-9]*) ;;
    *)
        echo "invalid version: ${version}" >&2
        exit 1
        ;;
esac

for project in python_lib services/api; do
    uv build --directory "${project}"
done

git tag -a "v${version}" -m "Release ${version}"
