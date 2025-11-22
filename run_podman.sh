#!/bin/bash

# Build the container image
echo "Building container image..."
podman build -t friendsgiving .

# Run the container
# --rm: Remove container when it stops
# -p 8080:8080: Map port 8080
# -v ...: Mount the menu.json file so updates are saved locally
echo "Starting container..."
podman run --rm -p 8080:8080 -v "$(pwd)/menu.json":/app/menu.json friendsgiving
