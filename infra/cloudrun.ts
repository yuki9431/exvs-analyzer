import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const gcsBucket = config.requireSecret("gcsBucket");

export const service = new gcp.cloudrunv2.Service(
  "exvs-analyzer",
  {
    location: gcp.config.region!,
    ingress: "INGRESS_TRAFFIC_ALL",
    template: {
      scaling: {
        maxInstanceCount: 3,
      },
      containers: [
        {
          // イメージはCDパイプラインが管理するため、初回importの値を保持
          image: "us-docker.pkg.dev/cloudrun/container/hello",
          ports: { containerPort: 8080 },
          envs: [
            {
              name: "GCS_BUCKET",
              value: gcsBucket,
            },
          ],
          resources: {
            limits: {
              cpu: "1000m",
              memory: "512Mi",
            },
          },
        },
      ],
      maxInstanceRequestConcurrency: 10,
    },
  },
  {
    // CDパイプラインがイメージを更新するため、Pulumiでは変更を無視する
    ignoreChanges: ["template.containers[0].image"],
  }
);

// 未認証アクセスを許可
export const iamBinding = new gcp.cloudrunv2.ServiceIamBinding(
  "exvs-analyzer-public",
  {
    name: service.name,
    location: gcp.config.region!,
    role: "roles/run.invoker",
    members: ["allUsers"],
  }
);

export const url = service.uri;
