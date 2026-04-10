import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const gcsBucket = config.requireSecret("gcsBucket");
const image = config.requireSecret("image");

export const service = new gcp.cloudrunv2.Service(
  "exvs-analyzer",
  {
    location: gcp.config.region!,
    ingress: "INGRESS_TRAFFIC_ALL",
    launchStage: "GA",
    template: {
      scaling: {
        maxInstanceCount: 3,
      },
      containers: [
        {
          image: image,
          ports: { containerPort: 8080, name: "http1" },
          envs: [
            {
              name: "GCS_BUCKET",
              value: gcsBucket,
            },
          ],
          resources: {
            cpuIdle: true,
            startupCpuBoost: true,
            limits: {
              cpu: "2",
              memory: "1Gi",
            },
          },
          startupProbe: {
            failureThreshold: 1,
            periodSeconds: 240,
            tcpSocket: {
              port: 8080,
            },
            timeoutSeconds: 240,
          },
        },
      ],
      maxInstanceRequestConcurrency: 10,
      timeout: "300s",
    },
    traffics: [
      {
        percent: 100,
        type: "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST",
      },
    ],
  },
  {
    ignoreChanges: ["client", "clientVersion"],
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
