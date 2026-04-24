#!/usr/bin/env bash

# Cleanup Docker container and image
function cleanup_webdav_container {
    echo "Stopping and removing WebDAV container..."
    docker stop webdav 2>/dev/null || true
    docker rm webdav 2>/dev/null || true
}

function cleanup_webdav_image {
    echo "Removing WebDAV test image..."
    docker rmi webdav-test 2>/dev/null || true
}
