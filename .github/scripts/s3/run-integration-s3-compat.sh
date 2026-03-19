#!/usr/bin/env bash
set -euo pipefail


# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"


# Source utils from the same directory
source "${script_dir}/utils.sh"

: "${access_key_id:?}"
: "${secret_access_key:?}"
: "${bucket_name:?}"
: "${s3_endpoint_host:?}"
: "${s3_endpoint_port:?}"
: "${label_filter:=s3-compatible}"

export ACCESS_KEY_ID=${access_key_id}
export SECRET_ACCESS_KEY=${secret_access_key}
export BUCKET_NAME=${bucket_name}
export S3_HOST=${s3_endpoint_host}
export S3_PORT=${s3_endpoint_port}

pushd "${repo_root}" > /dev/null
  echo -e "\n running tests with $(go version)..."
  echo "Selecting specs via label filter: ${label_filter}"
  ginkgo -r --label-filter="${label_filter}" s3/integration/
popd > /dev/null
