# インフラ管理 (Pulumi)

GCPリソースをPulumi (TypeScript) で管理する。2プロジェクト構成で環境を分離。

## ディレクトリ構成

```
infra/
├── shared/    ← 環境非依存リソース（API, IAM, DNS, GCS, Artifact Registry）
│   └── スタック: dev（固定）
└── app/       ← 環境ごとにデプロイするリソース（Cloud Run, ドメインマッピング）
    ├── スタック: prod
    └── スタック: staging
```

## 前提条件

- Docker（ローカル実行はMakefile経由）
- GCP認証 (`gcloud auth application-default login`)

## 初期セットアップ

```bash
# 1. shared スタック初期化
make pulumi-shared-install
make pulumi-shared-init

# 2. shared の設定値をセット
make pulumi-shared-shell
pulumi config set gcp:project <PROJECT_ID> --secret
pulumi config set exvs-shared:artifactRegistryRepo <REPO_NAME> --secret
pulumi config set exvs-shared:gcsBucket <BUCKET_NAME> --secret
# ... 他の secret も同様

# 3. app スタック初期化（prod / staging）
make pulumi-app-install
STACK=prod make pulumi-app-init
STACK=staging make pulumi-app-init

# 4. app の設定値をセット
STACK=prod make pulumi-app-shell
pulumi config set gcp:project <PROJECT_ID> --secret
pulumi config set exvs-app:image <IMAGE_URI> --secret
pulumi config set exvs-app:gcsBucket <BUCKET_NAME> --secret
```

## 日常操作

```bash
# shared プレビュー・適用
make pulumi-shared-preview
make pulumi-shared-up

# app プレビュー・適用（STACK で環境指定）
STACK=prod make pulumi-app-preview
STACK=staging make pulumi-app-preview
STACK=prod make pulumi-app-up
STACK=staging make pulumi-app-up
```

## デプロイフロー

```
コード変更 → mainマージ → staging に自動デプロイ
確認OK → gh workflow run cd.yml (environment=prod) → 本番デプロイ
```

## GitOps

| タイミング | ワークフロー | 内容 |
|-----------|-------------|------|
| PR作成/更新 | `infra-ci.yml` | shared + app (prod/staging) の `pulumi preview` |
| mainマージ | `cd.yml` | ビルド → staging に自動デプロイ |
| 手動実行 | `cd.yml` | 指定環境（prod/staging）にデプロイ |

## 管理対象リソース

### shared

| リソース | ファイル |
|---------|---------|
| 有効化API | `apis.ts` |
| Artifact Registry | `artifact-registry.ts` |
| GCSバケット | `storage.ts` |
| Cloud DNS ゾーン | `dns.ts` |
| IAM / WIF | `iam.ts` |
| 予算アラート | `budget.ts` |

### app

| リソース | ファイル |
|---------|---------|
| Cloud Run サービス | `index.ts` |
| ドメインマッピング | `index.ts` |
| CNAME レコード | `index.ts` |

## GitHub Secrets

| Secret | 説明 |
|--------|------|
| `PULUMI_STATE_BUCKET` | Pulumiステート保存用GCSバケット名 |
| `PULUMI_CONFIG_PASSPHRASE` | Pulumiスタックの暗号化パスフレーズ |
| `GCR_IMAGE` | Artifact RegistryのイメージURIベース |
| `WIF_PROVIDER` | Workload Identity Provider |
| `WIF_SERVICE_ACCOUNT` | GitHub Actions用サービスアカウント |
