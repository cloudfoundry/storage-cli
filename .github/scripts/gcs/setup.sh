#!/usr/bin/env bash
set -euo pipefail


# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"


: "${google_json_key_data:?}"

pushd "${script_dir}"
    source utils.sh
    gcloud_login
    make prep-gcs
popd
