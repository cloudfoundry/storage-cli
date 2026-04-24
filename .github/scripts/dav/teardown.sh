#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"

source "${script_dir}/utils.sh"

echo "Tearing down WebDAV test environment..."
cleanup_webdav_container
cleanup_webdav_image

echo "✓ Teardown complete"
