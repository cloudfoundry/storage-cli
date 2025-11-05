#!/usr/bin/env bash
set -euo pipefail


# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"

# Source utils from the same directory
source "${script_dir}/utils.sh"

: "${access_key_id:?}"
: "${secret_access_key:?}"
: "${region_name:=unset}"
: "${focus_regex:?}"
: "${assume_role_arn:=unset}"
: "${s3_endpoint_host:=unset}"


# Just need these to get the stack info
export AWS_ACCESS_KEY_ID=${access_key_id}
export AWS_SECRET_ACCESS_KEY=${secret_access_key}
export AWS_DEFAULT_REGION=${region_name}
export ASSUME_ROLE_ARN=${assume_role_arn}

# Some of these are optional
export ACCESS_KEY_ID=${access_key_id}
export SECRET_ACCESS_KEY=${secret_access_key}
export REGION=${region_name}
export S3_HOST=${s3_endpoint_host}

pushd "${release_dir}" > /dev/null
  echo -e "\n running tests with $(go version)..."
  scripts/ginkgo -r --focus="${focus_regex}" s3/integration/
popd > /dev/null
