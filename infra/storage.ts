import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const pulumiStateBucket = config.requireSecret("pulumiStateBucket");
const gcsBucket = config.requireSecret("gcsBucket");

// Pulumiステート保存用バケット
export const stateBucket = new gcp.storage.Bucket("pulumi-state", {
  name: pulumiStateBucket,
  location: gcp.config.region!,
  uniformBucketLevelAccess: true,
});

// アプリ用データバケット（ユーザーCSV保存）
export const dataBucket = new gcp.storage.Bucket("app-data", {
  name: gcsBucket,
  location: gcp.config.region!,
  uniformBucketLevelAccess: true,
});
