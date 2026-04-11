import { services } from "./apis";
import { repository } from "./artifact-registry";
import { service, url, iamBinding } from "./cloudrun";
import { nameServers, dnsZone, domainMapping, dnsRecords } from "./domain";
import { stateBucket, dataBucket } from "./storage";
// TODO: budget importはBilling Budget APIのquota project設定後に対応
// import { budget } from "./budget";

export const enabledApis = services.map((s) => s.service);
export const artifactRegistryId = repository.id;
export const cloudRunUrl = url;
export const cloudRunServiceName = service.name;
export const dnsNameServers = nameServers;
export const dnsZoneId = dnsZone.id;
export const customDomain = domainMapping.name;
export const requiredDnsRecords = dnsRecords;
export const pulumiStateBucketName = stateBucket.name;
export const appDataBucketName = dataBucket.name;
