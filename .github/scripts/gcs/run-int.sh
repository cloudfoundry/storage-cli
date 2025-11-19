#!/usr/bin/env bash
set -euo pipefail


# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"

: "${google_json_key_data:?}"

export GOOGLE_SERVICE_ACCOUNT="${google_json_key_data}"

pushd "${script_dir}" > /dev/null
    source utils.sh
    gcloud_login
popd > /dev/null

pushd "${script_dir}"

       # Set up conditional long test execution
    if [[ "${SKIP_LONG_TESTS:-}" == "yes" ]]; then
        make test-fast-int
    else
        make test-int
    fi
popd


