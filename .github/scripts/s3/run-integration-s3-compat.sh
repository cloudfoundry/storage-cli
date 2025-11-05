#!/usr/bin/env bash
set -euo pipefail


# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"

# Source utils from the same directory
source "${script_dir}/utils.sh"

: "${access_key_id:?}"
: "${secret_access_key:?}"
: "${bucket_name:?}"
: "${s3_endpoint_host:?}"
: "${s3_endpoint_port:?}"

export ACCESS_KEY_ID=${access_key_id}
export SECRET_ACCESS_KEY=${secret_access_key}
export BUCKET_NAME=${bucket_name}
export S3_HOST=${s3_endpoint_host}
export S3_PORT=${s3_endpoint_port}

pushd "${release_dir}" > /dev/null
  echo -e "\n running tests with $(go version)..."
  scripts/ginkgo -r --focus="S3 COMPATIBLE" s3/integration/
popd > /dev/null
