#!/usr/bin/env bash
set -euo pipefail

# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"

: "${DAV_ENDPOINT:?DAV_ENDPOINT environment variable must be set}"
: "${DAV_USER:?DAV_USER environment variable must be set}"
: "${DAV_PASSWORD:?DAV_PASSWORD environment variable must be set}"

echo "Running DAV integration tests..."
echo "  Endpoint: ${DAV_ENDPOINT}"
echo "  User: ${DAV_USER}"

pushd "${repo_root}/dav" > /dev/null
  echo -e "\nRunning tests with $(go version)..."
  ginkgo -v ./integration
popd > /dev/null
