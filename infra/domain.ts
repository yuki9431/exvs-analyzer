import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";
import { service } from "./cloudrun";

const config = new pulumi.Config();
const domain = config.require("domain");

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

export const dnsRecords = domainMapping.statuses.apply((statuses) =>
  (statuses || []).flatMap((s) =>
    (s.resourceRecords || []).map((r) => ({
      type: r.type,
      name: r.name,
      rrdata: r.rrdata,
    }))
  )
);
