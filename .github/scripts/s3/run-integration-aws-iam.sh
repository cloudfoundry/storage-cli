#!/usr/bin/env bash

# IAM Role Integration Test Flow:
#
# 1. Retrieve the S3 bucket name and IAM role ARN from the CloudFormation stack.
#
# 2. Build storage-cli and Ginkgo integration tests, then package them
#    with the Python Lambda handler into a ZIP file.
#
# 3. Create an AWS Lambda function using the ZIP and assign the IAM
#    execution role from CloudFormation.
#
# 4. Invoke Lambda synchronously. The Python handler runs Ginkgo tests,
#    which verify that storage-cli can access S3 using temporary IAM
#    role credentials provided through the Lambda environment.
#
# 5. Check the Lambda invocation response (StatusCode and FunctionError)
#    to determine whether the integration tests passed or failed.
#
# 6. Retrieve and print CloudWatch logs to show Ginkgo test output,
#    including failed assertions and debugging information.
#
# 7. On script exit, the EXIT trap attempts to delete the Lambda function
#    and its CloudWatch log group, warning if cleanup cannot be confirmed.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../../.." && pwd)"

source "${script_dir}/utils.sh"

: "${access_key_id:?}"
: "${secret_access_key:?}"
: "${region_name:?}"
: "${stack_name:?}"
: "${label_filter:?}"

export AWS_ACCESS_KEY_ID="${access_key_id}"
export AWS_SECRET_ACCESS_KEY="${secret_access_key}"
export AWS_REGION="${region_name}"
export AWS_DEFAULT_REGION="${region_name}"

# Read CloudFormation outputs
stack_info=$(get_stack_info "${stack_name}")
bucket_name=$(get_stack_info_of "${stack_info}" "BucketName")
iam_role_arn=$(get_stack_info_of "${stack_info}" "IamRoleArn")

# Validate required stack outputs
[[ -n "${bucket_name}" && "${bucket_name}" != "null" ]]
[[ -n "${iam_role_arn}" && "${iam_role_arn}" != "null" ]]

# Prepare Lambda payload
lambda_payload_json=$(jq -nc \
  --arg region "${region_name}" \
  --arg bucket "${bucket_name}" \
  --arg filter "${label_filter}" \
  '{
    region: $region,
    bucket_name: $bucket,
    s3_host: "s3.amazonaws.com",
    label_filter: $filter
  }')

lambda_payload_base64=$(
  printf '%s' "${lambda_payload_json}" | base64 | tr -d '\n'
)

# Register cleanup before creating AWS resources
lambda_function_name="s3cli-integration-$(date +%s)-${RANDOM}"
log_group_name="/aws/lambda/${lambda_function_name}"
lambda_log=$(mktemp -t "XXXXXX-lambda.log")

trap cleanup_lambda EXIT

cd "${repo_root}"

# Build test artifacts
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

echo "Building integration test artifacts..."

go build -o out/s3cli
ginkgo build s3/integration

zip -j payload.zip \
  s3/integration/integration.test \
  out/s3cli \
  "${script_dir}/assets/lambda_function.py"

# Create Lambda
echo "Creating Lambda: ${lambda_function_name}"

aws lambda create-function \
  --region "${region_name}" \
  --function-name "${lambda_function_name}" \
  --zip-file fileb://payload.zip \
  --role "${iam_role_arn}" \
  --timeout 600 \
  --memory-size 512 \
  --handler lambda_function.test_runner_handler \
  --runtime python3.9 > /dev/null

# Wait until Lambda is ready
echo "Waiting for Lambda to become active..."

aws lambda wait function-active-v2 \
  --region "${region_name}" \
  --function-name "${lambda_function_name}"

# Invoke synchronously
echo "Invoking Lambda integration tests..."

aws lambda invoke \
  --region "${region_name}" \
  --function-name "${lambda_function_name}" \
  --invocation-type RequestResponse \
  --log-type Tail \
  --payload "${lambda_payload_base64}" \
  --cli-read-timeout 900 \
  "${lambda_log}" > lambda_output.json

echo "Lambda invocation response:"
cat "${lambda_log}"
echo
echo "Lambda invocation completed."

# Check the invocation response
if jq -e '.StatusCode == 200 and .FunctionError == null' lambda_output.json > /dev/null; then
  echo "Lambda execution succeeded."
  test_failed=false
else
  echo "::error::Lambda reported an execution error."
  cat "${lambda_log}"
  test_failed=true
fi

# Retrieve CloudWatch logs
echo "Retrieving CloudWatch logs..."

# Create an empty log file, or truncate it if it already exists
: > lambda_output.log

max_attempts=30
stable_count=0
logs_complete=false

for ((attempt=1; attempt<=max_attempts; attempt++)); do
  echo "CloudWatch retrieval attempt ${attempt}/${max_attempts}"

  # Fetch all currently available pages across the log group.
  # The Lambda function name makes this log group unique.
  if aws logs filter-log-events \
    --region "${region_name}" \
    --log-group-name "${log_group_name}" \
    --output json > cloudwatch_response.json 2> cloudwatch_error.log; then

    jq -r '.events[].message' \
      cloudwatch_response.json > lambda_output.new

    # Require a REPORT marker and two consecutive
    # identical snapshots after it appears.
    if grep -q 'REPORT RequestId:' lambda_output.new; then
      if cmp -s lambda_output.log lambda_output.new; then
        stable_count=$((stable_count + 1))
      else
        stable_count=0
      fi
    else
      stable_count=0
    fi

    mv lambda_output.new lambda_output.log

    if (( stable_count >= 2 )); then
      logs_complete=true
      break
    fi
  else
    echo "CloudWatch logs not available yet:"
    cat cloudwatch_error.log
  fi

  sleep 2
done

echo "========== Lambda execution logs =========="
cat lambda_output.log
echo "==========================================="

if [[ "${logs_complete}" != true ]]; then
  echo "::warning title=Incomplete Lambda Logs::Could not retrieve all Lambda execution logs. \
  Some debugging information may be missing. \
  Function: '${lambda_function_name}', Region: '${region_name}'."
fi

if [[ "${test_failed}" == true ]]; then
  echo "::error title=Integration Test Failed::Lambda '${lambda_function_name}' reported an execution error."
  exit 1
fi

echo "IAM integration tests completed successfully."
