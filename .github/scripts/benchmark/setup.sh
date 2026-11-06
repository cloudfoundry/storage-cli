#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"


: "${benchmark_config_file:?}"
: "${provider:?provider is required (s3|gcs|azurebs|alioss)}"
CONFIG_FILE="${benchmark_config_file}"

case "${provider}" in

  s3)
    : "${aws_access_key_id:?}"
    : "${aws_secret_access_key:?}"
    : "${aws_region:=us-east-1}"
    : "${aws_stack_name:=s3cli-public-bucket}"

    # translate to names the s3 setup-aws-infrastructure.sh script expects
    export access_key_id="${aws_access_key_id}"
    export secret_access_key="${aws_secret_access_key}"
    export region_name="${aws_region}"
    export stack_name="${aws_stack_name}"

    bash "${script_dir}/../s3/setup-aws-infrastructure.sh"

    # source utils to use get_stack_info functions
    # re-export AWS vars since subprocess env does not propagate back
    export AWS_ACCESS_KEY_ID="${aws_access_key_id}"
    export AWS_SECRET_ACCESS_KEY="${aws_secret_access_key}"
    export AWS_DEFAULT_REGION="${aws_region}"
    source "${script_dir}/../s3/utils.sh"
    stack_info=$(get_stack_info "${aws_stack_name}")
    bucket_name=$(get_stack_info_of "${stack_info}" "BucketName")
    : "${bucket_name:?failed to read bucket name from stack outputs}"

    jq -n \
      --arg access_key_id     "${aws_access_key_id}" \
      --arg secret_access_key "${aws_secret_access_key}" \
      --arg bucket_name       "${bucket_name}" \
      --arg region            "${aws_region}" \
      '{
        access_key_id:     $access_key_id,
        secret_access_key: $secret_access_key,
        bucket_name:       $bucket_name,
        region:            $region
      }' > "${CONFIG_FILE}"
    ;;

  gcs)
    : "${google_json_key_data:?}"

    # login via gcloud
    source "${script_dir}/../gcs/utils.sh"
    gcloud_login

    # create the regional bucket
    make -C "${script_dir}/../gcs" regional-bucket

    # read bucket name from the lock file
    bucket_name="$(read_bucket_name "regional")"
    : "${bucket_name:?failed to read bucket name from gcs lock file}"

    jq -n \
      --arg json_key    "${google_json_key_data}" \
      --arg bucket_name "${bucket_name}" \
      '{
        provider:                    "Google",
        credentials_source:          "static",
        json_key:                    $json_key,
        bucket_name:                 $bucket_name,
        uniform_bucket_level_access: true
      }' > "${CONFIG_FILE}"
    ;;

  azurebs)
    : "${azure_storage_account:?}"
    : "${azure_storage_key:?}"
    export environment="AzureCloud"

    # create the container
    bash "${script_dir}/../azurebs/setup.sh"

    # read the container name from the lock file
    source "${script_dir}/../azurebs/utils.sh"
    container_name="$(read_container_name_from_file "${environment}")"
    : "${container_name:?failed to read container name from lock file}"

    jq -n \
      --arg account_name   "${azure_storage_account}" \
      --arg account_key    "${azure_storage_key}" \
      --arg container_name "${container_name}" \
      '{
        account_name:   $account_name,
        account_key:    $account_key,
        container_name: $container_name
      }' > "${CONFIG_FILE}"
    ;;

  alioss)
    : "${alioss_access_key_id:?}"
    : "${alioss_access_key_secret:?}"
    : "${alioss_region:=eu-central-1}"

    # translate to names the alioss script expects
    export access_key_id="${alioss_access_key_id}"
    export access_key_secret="${alioss_access_key_secret}"
    export region="${alioss_region}"
    export test_name="benchmark"

    # create the bucket
    bash "${script_dir}/../alioss/setup.sh"

    # read the bucket name from the lock file
    source "${script_dir}/../alioss/utils.sh"
    bucket_name="$(read_bucket_name_from_file "${test_name}")"
    : "${bucket_name:?failed to read bucket name from alioss lock file}"

    jq -n \
      --arg access_key_id     "${alioss_access_key_id}" \
      --arg access_key_secret "${alioss_access_key_secret}" \
      --arg bucket_name       "${bucket_name}" \
      --arg endpoint          "oss-${alioss_region}.aliyuncs.com" \
      '{
        access_key_id:     $access_key_id,
        access_key_secret: $access_key_secret,
        bucket_name:       $bucket_name,
        endpoint:          $endpoint
      }' > "${CONFIG_FILE}"
    ;;

  *)
    echo "Unknown provider: ${provider}. Must be one of: s3, gcs, azurebs, alioss"
    exit 1
    ;;
esac

echo "Setup complete. Config written to ${CONFIG_FILE}"
chmod 600 "${CONFIG_FILE}"
