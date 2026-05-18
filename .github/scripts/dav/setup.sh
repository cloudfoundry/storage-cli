#!/usr/bin/env bash
set -euo pipefail

# Get the directory where this script is located
script_dir="$( cd "$(dirname "${0}")" && pwd )"
repo_root="$(cd "${script_dir}/../../.." && pwd)"

source "${script_dir}/utils.sh"

# Cleanup any existing containers first
cleanup_webdav_container

echo "Building WebDAV test server Docker image..."
cd "${repo_root}/dav/integration/testdata"
docker build -t webdav-test .

echo "Starting WebDAV test server..."
docker run -d --name webdav -p 8443:443 webdav-test

# Wait for nginx to be ready
echo "Waiting for nginx to start..."
sleep 5

# Verify htpasswd file in container
echo "Verifying htpasswd file in container..."
docker exec webdav cat /etc/nginx/htpasswd

# Test connection
echo "Testing WebDAV server connection..."
if curl -k -u testuser:testpass -v https://localhost:8443/ 2>&1 | grep -q "200 OK\|301\|Authorization"; then
    echo "✓ WebDAV server is ready"
else
    echo "⚠ WebDAV server might not be fully ready yet"
fi
