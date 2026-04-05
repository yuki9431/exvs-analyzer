IMAGE_NAME := exvs-analyzer
PORT ?= 8080

.PHONY: build run restart stop test

## Docker イメージをビルド（キャッシュなし）
build:
	docker build --no-cache -t $(IMAGE_NAME) .

## コンテナを起動（localhost:$(PORT)）
run:
	@if docker ps -q -f name=$(IMAGE_NAME) | grep -q .; then \
		echo "Stopping existing container..."; \
		docker stop $(IMAGE_NAME) > /dev/null; \
	fi
	docker run --rm --name $(IMAGE_NAME) -p $(PORT):8080 $(IMAGE_NAME)

## ビルド後に起動（build + run）
restart: build run

## コンテナを停止
stop:
	docker stop $(IMAGE_NAME)

## Go テストを実行
test:
	docker run --rm -v "$(CURDIR)":/app -w /app golang:1.26-alpine go test ./internal/...
