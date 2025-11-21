#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"


: "${azure_storage_account:?}"
: "${azure_storage_key:?}"
: "${environment:=AzureCloud}"

export AZURE_STORAGE_ACCOUNT="${azure_storage_account}"
export AZURE_STORAGE_KEY="${azure_storage_key}"



pushd "${script_dir}"
    source utils.sh
    generate_container_name "${environment}"
    create_container "${environment}"
popd
