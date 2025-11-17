TMP_DIR="/tmp/storage-cli-azurebs-${GITHUB_RUN_ID:-${USER}}"

# generate a random container name with "azurebs-" prefix
function random_name {
    echo "azurebs-$(openssl rand -hex 20)"
}


# create a file with .lock suffix and store the container name inside it
function generate_container_name {
    local file_name="${1}.lock"
    local container_name="$(random_name)"
    mkdir -p "${TMP_DIR}"
    echo "${container_name}" > "${TMP_DIR}/${file_name}"
}


# retrieve the container name from the .lock file
function read_container_name_from_file {
    local file_name="$1"
    cat "${TMP_DIR}/${file_name}.lock"
}

# delete the .lock file
function delete_container_name_file {
    local file_name="$1"
    rm -f "${TMP_DIR}/${file_name}.lock"
}


function create_container {
    local container_name="$(read_container_name_from_file "$1")"
    
    az storage container create --account-name "${AZURE_STORAGE_ACCOUNT}" --account-key "${AZURE_STORAGE_KEY}" --name "${container_name}"

}


function delete_container {
    local container_name="$(read_container_name_from_file "$1")"
    
    az storage container delete --account-name "${AZURE_STORAGE_ACCOUNT}" --account-key "${AZURE_STORAGE_KEY}" --name "${container_name}"

}