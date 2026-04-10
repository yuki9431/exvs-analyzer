import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { service } from "./cloudrun";

const config = new pulumi.Config();
const domain = config.require("domain");
// www.exvs-analyzer.com → exvs-analyzer.com
const parts = domain.split(".");
const rootDomain = parts.slice(-2).join(".");

// Cloud DNS API有効化
export const dnsApi = new gcp.projects.Service("dns.googleapis.com", {
  service: "dns.googleapis.com",
  disableOnDestroy: false,
});

// Cloud DNSマネージドゾーン（ルートドメインで管理）
export const dnsZone = new gcp.dns.ManagedZone(
  "exvs-analyzer-zone",
  {
    name: "exvs-analyzer",
    dnsName: rootDomain + ".",
    description: "EXVS Analyzer DNS zone",
  },
  { dependsOn: [dnsApi] }
);

// Cloud Runドメインマッピング（wwwサブドメイン）
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

// DomainMappingが返すDNSレコード
export const dnsRecords = domainMapping.statuses.apply((statuses) => {
  const records = (statuses || []).flatMap((s) =>
    (s.resourceRecords || []).map((r) => ({
      type: r.type,
      name: r.name,
      rrdata: r.rrdata,
    }))
  );
  return records;
});

// ネームサーバー（お名前.comに設定する値）
export const nameServers = dnsZone.nameServers;
