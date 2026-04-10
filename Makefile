IMAGE_NAME := exvs-analyzer
PORT ?= 8080

.PHONY: build run restart stop test pulumi-install pulumi-init pulumi-preview pulumi-up

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

PULUMI_IMAGE := pulumi/pulumi:latest
PULUMI_STATE_BUCKET ?= exvs2ib-analyzer-pulumi-state
PULUMI_LOGIN = pulumi login gs://$(PULUMI_STATE_BUCKET) && pulumi stack select dev
PULUMI_RUN = docker run --rm --entrypoint "" \
	-v "$(CURDIR)/infra":/infra \
	-v "$(HOME)/.config/gcloud":/root/.config/gcloud \
	-w /infra \
	-e PULUMI_CONFIG_PASSPHRASE \
	-e CLOUDSDK_CORE_PROJECT=$$(gcloud config get-value project 2>/dev/null) \
	-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
	$(PULUMI_IMAGE)

## Pulumi依存パッケージをインストール
pulumi-install:
	$(PULUMI_RUN) npm install

## Pulumiバックエンドにログイン＆スタック初期化
pulumi-init:
	$(PULUMI_RUN) sh -c "pulumi login gs://$(PULUMI_STATE_BUCKET) && pulumi stack init dev || pulumi stack select dev"

## インフラ変更のプレビュー
pulumi-preview:
	$(PULUMI_RUN) sh -c "$(PULUMI_LOGIN) && pulumi preview"

## インフラ変更を適用
pulumi-up:
	$(PULUMI_RUN) sh -c "$(PULUMI_LOGIN) && pulumi up"
