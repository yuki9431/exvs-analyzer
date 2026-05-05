import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const pulumiStateBucket = config.requireSecret("pulumiStateBucket");
const gcsBucket = config.requireSecret("gcsBucket");
const computeSa = config.requireSecret("computeSa");
const ownerEmail = config.requireSecret("ownerEmail");

// Pulumiステート保存用バケット
export const stateBucket = new gcp.storage.Bucket("pulumi-state", {
  name: pulumiStateBucket,
  location: gcp.config.region!,
  uniformBucketLevelAccess: true,
  publicAccessPrevention: "enforced",
  forceDestroy: false,
});

// アプリ用データバケット（ユーザーCSV保存）
export const dataBucket = new gcp.storage.Bucket("app-data", {
  name: gcsBucket,
  location: gcp.config.region!,
  uniformBucketLevelAccess: true,
  publicAccessPrevention: "enforced",
  versioning: { enabled: true },
  forceDestroy: false,
  lifecycleRules: [
    {
      action: { type: "Delete" },
      condition: {
        numNewerVersions: 3,
        withState: "ARCHIVED",
      },
    },
  ],
});

// オーナーアカウントにデータバケットの管理権限を付与
export const dataBucketOwnerIam = new gcp.storage.BucketIAMMember(
  "app-data-owner",
  {
    bucket: dataBucket.name,
    role: "roles/storage.admin",
    member: pulumi.interpolate`user:${ownerEmail}`,
  }
);

// Cloud Runデフォルトcompute SAにデータバケットへの最低限の権限を付与
export const dataBucketIam = new gcp.storage.BucketIAMMember(
  "app-data-compute-sa",
  {
    bucket: dataBucket.name,
    role: "roles/storage.objectUser",
    member: pulumi.interpolate`serviceAccount:${computeSa}`,
  }
);
