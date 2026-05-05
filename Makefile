IMAGE_NAME := exvs-analyzer
PORT ?= 8080

.PHONY: build run restart stop test \
	pulumi-shared-install pulumi-shared-init pulumi-shared-preview pulumi-shared-shell \
	pulumi-app-install pulumi-app-init pulumi-app-preview pulumi-app-shell

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
STACK ?= prod

# shared用（スタック固定: dev）
PULUMI_SHARED_LOGIN = pulumi login gs://$(PULUMI_STATE_BUCKET) && pulumi stack select shared
PULUMI_SHARED_RUN = docker run --rm --entrypoint "" \
	-v "$(CURDIR)/infra/shared":/infra \
	-v "$(HOME)/.config/gcloud":/root/.config/gcloud \
	-w /infra \
	-e PULUMI_CONFIG_PASSPHRASE \
	-e CLOUDSDK_CORE_PROJECT=$$(gcloud config get-value project 2>/dev/null) \
	-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
	$(PULUMI_IMAGE)

# app用（STACK変数で環境切り替え: prod / stg）
PULUMI_APP_LOGIN = pulumi login gs://$(PULUMI_STATE_BUCKET) && pulumi stack select $(STACK)
PULUMI_APP_RUN = docker run --rm --entrypoint "" \
	-v "$(CURDIR)/infra/app":/infra \
	-v "$(HOME)/.config/gcloud":/root/.config/gcloud \
	-w /infra \
	-e PULUMI_CONFIG_PASSPHRASE \
	-e CLOUDSDK_CORE_PROJECT=$$(gcloud config get-value project 2>/dev/null) \
	-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
	$(PULUMI_IMAGE)

## === shared（環境非依存リソース） ===

## shared: 依存パッケージをインストール
pulumi-shared-install:
	$(PULUMI_SHARED_RUN) npm install

## shared: バックエンドにログイン＆スタック初期化
pulumi-shared-init:
	$(PULUMI_SHARED_RUN) sh -c "pulumi login gs://$(PULUMI_STATE_BUCKET) && pulumi stack init shared || pulumi stack select shared"

## shared: インフラ変更のプレビュー
pulumi-shared-preview:
	$(PULUMI_SHARED_RUN) sh -c "$(PULUMI_SHARED_LOGIN) && pulumi preview"

## shared: シェルで入る（pulumi up はここで実行）
pulumi-shared-shell:
	docker run --rm -it --entrypoint "" \
		-v "$(CURDIR)/infra/shared":/infra \
		-v "$(HOME)/.config/gcloud":/root/.config/gcloud \
		-w /infra \
		-e PULUMI_CONFIG_PASSPHRASE \
		-e CLOUDSDK_CORE_PROJECT=$$(gcloud config get-value project 2>/dev/null) \
		-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
		$(PULUMI_IMAGE) \
		sh -c "$(PULUMI_SHARED_LOGIN) && sh"

## === app（環境ごとにデプロイ: STACK=prod|stg） ===

## app: 依存パッケージをインストール
pulumi-app-install:
	$(PULUMI_APP_RUN) npm install

## app: バックエンドにログイン＆スタック初期化
pulumi-app-init:
	$(PULUMI_APP_RUN) sh -c "pulumi login gs://$(PULUMI_STATE_BUCKET) && pulumi stack init $(STACK) || pulumi stack select $(STACK)"

## app: インフラ変更のプレビュー（STACK=prod|stg）
pulumi-app-preview:
	$(PULUMI_APP_RUN) sh -c "$(PULUMI_APP_LOGIN) && pulumi preview"

## app: シェルで入る（pulumi up はここで実行。STACK=prod|stg）
pulumi-app-shell:
	docker run --rm -it --entrypoint "" \
		-v "$(CURDIR)/infra/app":/infra \
		-v "$(HOME)/.config/gcloud":/root/.config/gcloud \
		-w /infra \
		-e PULUMI_CONFIG_PASSPHRASE \
		-e CLOUDSDK_CORE_PROJECT=$$(gcloud config get-value project 2>/dev/null) \
		-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
		$(PULUMI_IMAGE) \
		sh -c "$(PULUMI_APP_LOGIN) && sh"
