#!/bin/bash
set -e

IMAGE_NAME="exvs-analyzer"
PORT="${PORT:-8080}"

usage() {
  echo "Usage: $0 {build|run|restart}"
  echo ""
  echo "Commands:"
  echo "  build    Dockerイメージをビルド（キャッシュなし）"
  echo "  run      コンテナを起動（localhost:${PORT}）"
  echo "  restart  ビルド後に起動（build + run）"
  exit 1
}

do_build() {
  echo "Building ${IMAGE_NAME}..."
  docker build --no-cache -t "${IMAGE_NAME}" .
  echo "Build complete."
}

do_run() {
  # 既存コンテナがあれば停止
  if docker ps -q -f name="${IMAGE_NAME}" | grep -q .; then
    echo "Stopping existing container..."
    docker stop "${IMAGE_NAME}" >/dev/null
  fi

  echo "Starting ${IMAGE_NAME} on port ${PORT}..."
  docker run --rm --name "${IMAGE_NAME}" -p "${PORT}:8080" "${IMAGE_NAME}"
}

case "${1}" in
  build)   do_build ;;
  run)     do_run ;;
  restart) do_build && do_run ;;
  *)       usage ;;
esac
