#!/usr/bin/env bash

# Possible args
# --build: Build the container image
# --host: Use host networking
# -h/--help: Show help message

show_help() {
    echo "Usage: $0 [--build] [--host] [-h|--help]"
    echo
    echo "Options:"
    echo "  --build       Build the container image"
    echo "  --host        Use host networking"
    echo "  -d, --detach  Run the container in detached mode"
    echo "  -h, --help    Show this help message"
}

# Parse arguments
for arg in "$@"; do
    case $arg in
        --build)
        BUILD_IMAGE=1
        shift
        ;;
        --host)
        USE_HOST_NETWORK=1
        shift
        ;;
        -d|--detach)
        DETACH_MODE=1
        shift
        ;;
        -h|--help)
        show_help
        exit 0
        ;;
        *)
        ;;
    esac
done



# Build the container image if --build is passed as the first argument
if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
    echo "Building container image..."
    podman build -t friendsgiving .
else
    echo "Pulling latest container image..."
    podman pull ghcr.io/wokuno/friendsgiving:latest
fi

# Run the container
# --rm: Remove container when it stops
# -p 0.0.0.0:8080:8080: Map port 8080 on all interfaces when using bridge networking
# -v ...: Mount the menu.json file so updates are saved locally
# Set USE_HOST_NETWORK=1 to use --network host so 192.168.9.187 can reach the container (requires root)
echo "Starting container..."
NETWORK_ARGS="-p 0.0.0.0:8080:8000"
if [[ "${USE_HOST_NETWORK:-0}" == "1" ]]; then
	NETWORK_ARGS="--network host"
	echo "Switching to host networking so 192.168.9.187:8080 can reach the server."
fi

DETACH_ARG=""
if [[ "${DETACH_MODE:-0}" == "1" ]]; then
    DETACH_ARG="-d"
    echo "Running container in detached mode."
fi

podman run $DETACH_ARG --rm $NETWORK_ARGS -v "$(pwd)/src/data/menu.json":/app/data/menu.json friendsgiving
