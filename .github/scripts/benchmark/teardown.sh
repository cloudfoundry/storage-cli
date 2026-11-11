#!/usr/bin/env bash
set -euo pipefail

script_dir="$( cd "$(dirname "${0}")" && pwd )"

: "${benchmark_config_file:?}"
: "${provider:?provider is required (s3|gcs|azurebs|alioss)}"
CONFIG_FILE="${benchmark_config_file}"

# always remove sensitive files even if teardown fails
trap 'rm -f "${CONFIG_FILE}" "/tmp/gcp-key.json"' EXIT

case "${provider}" in

  s3)
    : "${aws_access_key_id:?}"
    : "${aws_secret_access_key:?}"
    : "${aws_region:=us-east-1}"
    : "${aws_stack_name:=s3cli-benchmark}"

    # translate to names the s3 teardown script expects
    export access_key_id="${aws_access_key_id}"
    export secret_access_key="${aws_secret_access_key}"
    export region_name="${aws_region}"
    export stack_name="${aws_stack_name}"

    bash "${script_dir}/../s3/teardown-infrastructure.sh"
    ;;

  gcs)
    : "${google_json_key_data:?}"

    source "${script_dir}/../gcs/utils.sh"
    gcloud_login

    # read bucket name and delete — cannot use make clean-gcs since it
    # requires all three lock files (regional, multiregional, public)
    # but we only created the regional bucket
    bucket_name="$(read_bucket_name "regional")"
    : "${bucket_name:?failed to read bucket name from gcs lock file}"

    gsutil rm -r "gs://${bucket_name}" || true
    ;;

  azurebs)
    : "${azure_storage_account:?}"
    : "${azure_storage_key:?}"
    export environment="AzureCloud"

    bash "${script_dir}/../azurebs/teardown.sh"
    ;;

  alioss)
    : "${alioss_access_key_id:?}"
    : "${alioss_access_key_secret:?}"
    : "${alioss_region:=eu-central-1}"

    # translate to names the alioss teardown script expects
    export access_key_id="${alioss_access_key_id}"
    export access_key_secret="${alioss_access_key_secret}"
    export region="${alioss_region}"
    export test_name="benchmark"

    bash "${script_dir}/../alioss/teardown.sh"
    ;;

  *)
    echo "Unknown provider: ${provider}. Must be one of: s3, gcs, azurebs, alioss"
    exit 1
    ;;
esac

echo "Teardown complete."
