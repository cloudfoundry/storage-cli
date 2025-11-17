#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"

: "${azure_storage_account:?}"
: "${azure_storage_key:?}"
: "${environment:=AzureCloud}"

export ACCOUNT_NAME="${azure_storage_account}"
export ACCOUNT_KEY="${azure_storage_key}"
export ENVIRONMENT="${environment}"

pushd "${script_dir}" > /dev/null
    source utils.sh
    container_name="$(read_container_name_from_file "${environment}")"
    : "${container_name:?}"
    export CONTAINER_NAME="${container_name}"
popd > /dev/null

pushd "${repo_root}" > /dev/null
  echo -e "\n running tests with $(go version)..."
  ginkgo -r azurebs/integration/
popd > /dev/null

