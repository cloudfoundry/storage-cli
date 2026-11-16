#!/usr/bin/env bash

get_stack_info() {
  local stack_name=$1

  aws cloudformation describe-stacks \
  | jq --arg stack_name "${stack_name}" '.Stacks[] | select(.StackName=="\($stack_name)")'
}

get_stack_info_of() {
  local stack_info=$1
  local key=$2
  echo "${stack_info}" | jq -r --arg key "${key}" '.Outputs[] | select(.OutputKey=="\($key)").OutputValue'
}

get_stack_status() {
  local stack_name=$1

  get_stack_info "${stack_name}" | jq -r '.StackStatus'
}


cleanup_lambda() {
local exit_code=$?
echo "Cleaning up Lambda: ${lambda_function_name}"

local deleted=false
local delete_accepted=false
local error

if error=$(aws lambda delete-function \
  --region "${region_name}" \
  --function-name "${lambda_function_name}" 2>&1); then
  delete_accepted=true
elif [[ "${error}" == *"ResourceNotFoundException"* ]]; then
  deleted=true
else
  echo "Initial Lambda deletion failed: ${error}"
fi

for attempt in {1..5}; do
  [[ "${deleted}" == true ]] && break

  if error=$(aws lambda get-function \
    --region "${region_name}" \
    --function-name "${lambda_function_name}" 2>&1); then

    echo "Lambda still exists (check ${attempt}/5)."

    # Retry deletion only if no previous request succeeded
    if [[ "${delete_accepted}" == false ]]; then
      if aws lambda delete-function \
        --region "${region_name}" \
        --function-name "${lambda_function_name}"; then
        delete_accepted=true
      fi
    fi

  elif [[ "${error}" == *"ResourceNotFoundException"* ]]; then
    echo "Lambda deletion confirmed."
    deleted=true
    break

  else
    echo "Unable to verify Lambda state: ${error}"
  fi

  if [[ "${attempt}" -lt 5 ]]; then
    sleep 3
  fi
done


if [[ "${deleted}" == false ]]; then
  echo "::warning title=Potential AWS Resource Leak::Lambda '${lambda_function_name}' in '${region_name}' may still exist. \
Automatic cleanup failed or could not be verified."
fi


aws logs delete-log-group \
  --region "${region_name}" \
  --log-group-name "/aws/lambda/${lambda_function_name}" || true

rm -f "${lambda_log}"

exit "${exit_code}"
}
