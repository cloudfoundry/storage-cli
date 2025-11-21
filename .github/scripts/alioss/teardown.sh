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
    aliyun_configure
    if delete_bucket "${test_name}" ; then
        delete_bucket_name_file "${test_name}"
    else
        exit 1
    fi
popd
