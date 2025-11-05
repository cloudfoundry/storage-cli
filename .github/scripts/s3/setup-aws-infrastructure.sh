#!/usr/bin/env bash
set -euo pipefail

# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"

# Source utils from the same directory
source "${script_dir}/utils.sh"

: "${access_key_id:?}"
: "${secret_access_key:?}"
: "${region_name:?}"
: "${stack_name:?}"

export AWS_ACCESS_KEY_ID=${access_key_id}
export AWS_SECRET_ACCESS_KEY=${secret_access_key}
export AWS_DEFAULT_REGION=${region_name}

if [ -n "${role_arn:-}" ]; then
  export AWS_ROLE_ARN=${role_arn}
  aws configure --profile creds_account set aws_access_key_id "${AWS_ACCESS_KEY_ID}"
  aws configure --profile creds_account set aws_secret_access_key "${AWS_SECRET_ACCESS_KEY}"
  aws configure --profile resource_account set source_profile "creds_account"
  aws configure --profile resource_account set role_arn "${AWS_ROLE_ARN}"
  aws configure --profile resource_account set region "${AWS_DEFAULT_REGION}"
  unset AWS_ACCESS_KEY_ID
  unset AWS_SECRET_ACCESS_KEY
  unset AWS_DEFAULT_REGION
  export AWS_PROFILE=resource_account
fi

cmd="aws cloudformation create-stack \
    --stack-name    ${stack_name} \
    --template-body file://${script_dir}/assets/cloudformation-${stack_name}.template.json \
    --capabilities  CAPABILITY_IAM"
echo "Running: ${cmd}"; ${cmd}

while true; do
  stack_status=$(get_stack_status "${stack_name}")
  echo "StackStatus ${stack_status}"
  if [ "${stack_status}" == 'CREATE_IN_PROGRESS' ]; then
    echo "sleeping 5s"; sleep 5s
  else
    break
  fi
done

if [ "${stack_status}" != 'CREATE_COMPLETE' ]; then
  echo "cloudformation failed stack info:"
  get_stack_info "${stack_name}"
  exit 1
fi
