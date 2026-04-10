import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { service } from "./cloudrun";

const config = new pulumi.Config();
const domain = config.require("domain");
const domainVerificationTxt = config.require("domainVerificationTxt");
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

// wwwサブドメインのCNAMEレコード（Cloud Runのドメインマッピング用）
export const cnameRecord = new gcp.dns.RecordSet("www-cname", {
  managedZone: dnsZone.name,
  name: domain + ".",
  type: "CNAME",
  ttl: 300,
  rrdatas: ["ghs.googlehosted.com."],
});

// ドメイン所有権確認用TXTレコード
export const verificationTxt = new gcp.dns.RecordSet("domain-verification", {
  managedZone: dnsZone.name,
  name: rootDomain + ".",
  type: "TXT",
  ttl: 300,
  rrdatas: [`"${domainVerificationTxt}"`],
});

// ネームサーバー（お名前.comに設定する値）
export const nameServers = dnsZone.nameServers;

// DomainMappingが返すDNSレコード
export const dnsRecords = domainMapping.statuses.apply((statuses) =>
  (statuses || []).flatMap((s) =>
    (s.resourceRecords || []).map((r) => ({
      type: r.type,
      name: r.name,
      rrdata: r.rrdata,
    }))
  )
);
