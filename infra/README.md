# インフラ管理 (Pulumi)

GCPリソースをPulumi (TypeScript) で管理する。

## 前提条件

- [Pulumi CLI](https://www.pulumi.com/docs/install/)
- Node.js 22+
- GCP認証 (`gcloud auth application-default login`)

## 初期セットアップ

```bash
# 1. 依存パッケージをインストール
cd infra && npm install

# 2. GCSバックエンドにログイン（バケット名はGitHub Secretsを参照）
pulumi login gs://<PULUMI_STATE_BUCKET>

# 3. スタックを作成
pulumi stack init dev

# 4. 設定値をセット
pulumi config set gcp:project <PROJECT_ID> --secret
pulumi config set gcp:region asia-northeast1
pulumi config set exvs-analyzer:artifactRegistryRepo <REPO_NAME> --secret
pulumi config set exvs-analyzer:gcsBucket <BUCKET_NAME> --secret
pulumi config set exvs-analyzer:billingAccount <BILLING_ACCOUNT_ID> --secret
pulumi config set exvs-analyzer:budgetAmount <AMOUNT>  --secret
```

## 既存リソースのimport

初回のみ、既存GCPリソースをPulumi管理下に取り込む。

```bash
# API有効化
pulumi import gcp:projects/service:Service run.googleapis.com <PROJECT_ID>/run.googleapis.com
pulumi import gcp:projects/service:Service artifactregistry.googleapis.com <PROJECT_ID>/artifactregistry.googleapis.com
pulumi import gcp:projects/service:Service cloudbuild.googleapis.com <PROJECT_ID>/cloudbuild.googleapis.com

# Artifact Registry
pulumi import gcp:artifactregistry/repository:Repository <REPO_NAME> projects/<PROJECT_ID>/locations/asia-northeast1/repositories/<REPO_NAME>

# Cloud Run
pulumi import gcp:cloudrunv2/service:Service exvs-analyzer projects/<PROJECT_ID>/locations/asia-northeast1/services/exvs-analyzer

# 予算アラート
pulumi import gcp:billing/budget:Budget monthly-budget <BUDGET_ID>
```

import後に `pulumi preview` で差分がゼロになるまでコードを調整する。

## 日常操作

```bash
# プレビュー
make pulumi-preview

# 適用
make pulumi-up
```

## GitOps

`infra/` 配下のファイルを変更してPRを出すと、自動で以下が実行される。

| タイミング | ワークフロー | 内容 |
|-----------|-------------|------|
| PR作成/更新 | `infra-ci.yml` | `pulumi preview` を実行し、結果をPRコメントに投稿 |
| mainマージ | `infra-cd.yml` | `pulumi up` を実行し、インフラを自動適用 |

## 管理対象リソース

| リソース | ファイル |
|---------|---------|
| Cloud Run サービス | `cloudrun.ts` |
| Artifact Registry | `artifact-registry.ts` |
| 有効化API | `apis.ts` |
| 予算アラート | `budget.ts` |

## GitHub Secrets（追加が必要）

| Secret | 説明 |
|--------|------|
| `PULUMI_STATE_BUCKET` | Pulumiステート保存用GCSバケット名 |
| `PULUMI_CONFIG_PASSPHRASE` | Pulumiスタックの暗号化パスフレーズ |
