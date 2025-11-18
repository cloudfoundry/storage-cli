#!/usr/bin/env bash

TMP_DIR="/tmp/storage-cli-alioss-${GITHUB_RUN_ID:-${USER}}"

# generate a random bucket name with "alioss-" prefix
function random_name {
    echo "alioss-$(openssl rand -hex 20)"
}


# create a file with .lock suffix and store the bucket name inside it
function generate_bucket_name {
    local file_name="${1}.lock"
    local bucket_name="$(random_name)"
    mkdir -p "${TMP_DIR}"
    echo "${bucket_name}" > "${TMP_DIR}/${file_name}"
}


# retrieve the bucket name from the .lock file
function read_bucket_name_from_file {
    local file_name="$1"
    cat "${TMP_DIR}/${file_name}.lock"
}

# delete the .lock file
function delete_bucket_name_file {
    local file_name="$1"
    rm -f "${TMP_DIR}/${file_name}.lock"
}


function aliyun_configure {
    aliyun configure set --profile "$ALI_PROFILE" \
    --access-key-id "$ALI_ACCESS_KEY_ID" \
    --access-key-secret "$ALI_ACCESS_KEY_SECRET" \
    --region "$ALI_REGION"   
}



function create_bucket {
    local bucket_name="$(read_bucket_name_from_file "$1")"
    aliyun --profile "$ALI_PROFILE" oss mb "oss://$bucket_name"
}


function delete_bucket {
    local bucket_name="$(read_bucket_name_from_file "$1")"
    aliyun --profile "$ALI_PROFILE" oss rb "oss://$bucket_name" --force
}