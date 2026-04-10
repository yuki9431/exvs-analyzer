import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

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

// ネームサーバー（お名前.comに設定する値）
export const nameServers = dnsZone.nameServers;

// TODO: Step 2 - お名前.comでNSレコード設定後にDomainMappingを有効化
// import { service } from "./cloudrun";
// export const domainMapping = new gcp.cloudrun.DomainMapping(...)
