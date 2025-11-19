#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"



: "${access_key_id:?}"
: "${access_key_secret:?}"
: "${test_name:=general}"
: "${region:=eu-central-1}"


export ALI_ACCESS_KEY_ID="${access_key_id}"
export ALI_ACCESS_KEY_SECRET="${access_key_secret}"
export ALI_REGION="${region}"



pushd "${script_dir}"
    source utils.sh
    generate_bucket_name "${test_name}"
    aliyun_configure
    create_bucket "${test_name}"
popd
