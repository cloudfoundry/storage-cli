#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"



: "${access_key_id:?}"
: "${access_key_secret:?}"
: "${profile:=integration-tests}"
: "${test_name:=general}"
: "${region:=cn-hangzhou}"


export ALI_ACCESS_KEY_ID="${access_key_id}"
export ALI_ACCESS_KEY_SECRET="${secret_access_key}"
export ALI_REGION="${region}"
export ALI_PROFILE="${profile}"



pushd "${script_dir}"
    source utils.sh
    generate_bucket_name "${test_name}"
    aliyun_configure
    create_bucket "${test_name}"
popd
