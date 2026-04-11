import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { stateBucket, dataBucket } from "./storage";
import { services } from "./apis";

const config = new pulumi.Config();
const githubRepo = config.require("githubRepo");

// GitHub Actions用サービスアカウント
export const githubActionsSa = new gcp.serviceaccount.Account(
  "github-actions",
  {
    accountId: "github-actions",
    displayName: "GitHub Actions",
  }
);

// Workload Identity Pool
export const wifPool = new gcp.iam.WorkloadIdentityPool("github-pool", {
  workloadIdentityPoolId: "github-pool",
  displayName: "GitHub Actions Pool",
});

// Workload Identity Provider（OIDC）
export const wifProvider = new gcp.iam.WorkloadIdentityPoolProvider(
  "github-provider",
  {
    workloadIdentityPoolId: wifPool.workloadIdentityPoolId,
    workloadIdentityPoolProviderId: "github-provider",
    displayName: "GitHub Provider",
    attributeMapping: {
      "google.subject": "assertion.sub",
      "attribute.repository": "assertion.repository",
    },
    attributeCondition: `assertion.repository=='${githubRepo}'`,
    oidc: {
      issuerUri: "https://token.actions.githubusercontent.com",
    },
  }
);

// WIF → サービスアカウントへのworkloadIdentityUser権限
export const wifBinding = new gcp.serviceaccount.IAMBinding(
  "github-actions-wif",
  {
    serviceAccountId: githubActionsSa.name,
    role: "roles/iam.workloadIdentityUser",
    members: [
      pulumi.interpolate`principalSet://iam.googleapis.com/${wifPool.name}/attribute.repository/${githubRepo}`,
    ],
  }
);

// サービスアカウント自身のserviceAccountUser権限
export const saUserBinding = new gcp.serviceaccount.IAMBinding(
  "github-actions-sa-user",
  {
    serviceAccountId: githubActionsSa.name,
    role: "roles/iam.serviceAccountUser",
    members: [githubActionsSa.member],
  }
);

// プロジェクトレベルのIAMロール（最小権限）
const projectRoles = [
  "roles/cloudbuild.builds.editor",
  "roles/run.developer",
  "roles/viewer",
];

export const projectBindings = projectRoles.map(
  (role) =>
    new gcp.projects.IAMMember(
      `github-actions-${role.split("/")[1]}`,
      {
        project: gcp.config.project!,
        role: role,
        member: githubActionsSa.member,
      },
      { dependsOn: services }
    )
);

// バケット単位のStorage権限（プロジェクトレベルのstorage.adminを廃止）
export const stateBucketBinding = new gcp.storage.BucketIAMMember(
  "github-actions-state-bucket",
  {
    bucket: stateBucket.name,
    role: "roles/storage.objectAdmin",
    member: githubActionsSa.member,
  }
);

export const dataBucketBinding = new gcp.storage.BucketIAMMember(
  "github-actions-data-bucket",
  {
    bucket: dataBucket.name,
    role: "roles/storage.objectAdmin",
    member: githubActionsSa.member,
  }
);
