#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"

: "${access_key_id:?}"
: "${access_key_secret:?}"
: "${test_name:=general}"
: "${region:=eu-central-1}"

export ACCESS_KEY_ID="${access_key_id}"
export ACCESS_KEY_SECRET="${secret_access_key}"

pushd "${script_dir}" > /dev/null
    source utils.sh
    bucket_name="$(read_bucket_name_from_file "${test_name}")"
    : "${bucket_name:?}"
    export BUCKET_NAME="${bucket_name}"
popd > /dev/null

export ENDPOINT="https://${BUCKET_NAME}.${region}.aliyuncs.com"

pushd "${repo_root}" > /dev/null
  echo -e "\n running tests with $(go version)..."
  ginkgo -r alioss/integration/
popd > /dev/null

