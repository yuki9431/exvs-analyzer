import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const serviceName = config.require("serviceName");
const cpu = config.require("cpu");
const memory = config.require("memory");
const maxInstances = config.requireNumber("maxInstances");
const image = config.requireSecret("image");
const domain = config.require("domain");
const gcsBucket = config.requireSecret("gcsBucket");

// shared スタックからDNSゾーン名を取得
const sharedStackName = config.require("sharedStack");
const shared = new pulumi.StackReference(sharedStackName);
const dnsZoneName = shared.getOutput("dnsZoneName") as pulumi.Output<string>;

// Cloud Run サービス
export const service = new gcp.cloudrunv2.Service(
  serviceName,
  {
    name: serviceName,
    location: gcp.config.region!,
    ingress: "INGRESS_TRAFFIC_ALL",
    launchStage: "GA",
    template: {
      scaling: {
        maxInstanceCount: maxInstances,
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
              cpu: cpu,
              memory: memory,
            },
          },
          startupProbe: {
            failureThreshold: 5,
            periodSeconds: 10,
            tcpSocket: {
              port: 8080,
            },
            timeoutSeconds: 5,
          },
          livenessProbe: {
            httpGet: {
              path: "/health",
              port: 8080,
            },
            periodSeconds: 30,
            failureThreshold: 3,
            timeoutSeconds: 5,
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

// Cloud Runドメインマッピング
export const domainMapping = new gcp.cloudrun.DomainMapping(
  "exvs-analyzer-domain",
  {
    location: gcp.config.region!,
    name: domain,
    metadata: {
      namespace: gcp.config.project!,
    },
    spec: {
      routeName: service.name,
    },
  }
);

// サブドメインのCNAMEレコード（Cloud Runのドメインマッピング用）
export const cnameRecord = new gcp.dns.RecordSet("cname", {
  managedZone: dnsZoneName,
  name: domain + ".",
  type: "CNAME",
  ttl: 300,
  rrdatas: ["ghs.googlehosted.com."],
});

export const url = service.uri;
export const cloudRunServiceName = service.name;
export const customDomain = domainMapping.name;
