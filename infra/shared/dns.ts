import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { services as enabledApis } from "./apis";

const config = new pulumi.Config();
const domain = config.require("domain");
const domainVerificationTxt = config.require("domainVerificationTxt");

// Cloud DNSマネージドゾーン（ルートドメインで管理）
export const dnsZone = new gcp.dns.ManagedZone(
  "exvs-analyzer-zone",
  {
    name: "exvs-analyzer",
    dnsName: domain + ".",
    description: "EXVS Analyzer DNS zone",
    dnssecConfig: {
      state: "on",
    },
  },
  { dependsOn: enabledApis }
);

// ドメイン所有権確認用TXTレコード
export const verificationTxt = new gcp.dns.RecordSet("domain-verification", {
  managedZone: dnsZone.name,
  name: domain + ".",
  type: "TXT",
  ttl: 300,
  rrdatas: [`"${domainVerificationTxt}"`],
});

// ネームサーバー（お名前.comに設定する値）
export const nameServers = dnsZone.nameServers;
