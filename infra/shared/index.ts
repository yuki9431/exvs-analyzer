import { services } from "./apis";
import { repository } from "./artifact-registry";
import { dnsZone, nameServers } from "./dns";
import { stateBucket, dataBucket } from "./storage";
import { githubActionsSa, wifPool, wifProvider } from "./iam";
// TODO: budget importはBilling Budget APIのquota project設定後に対応
// import { budget } from "./budget";

// app スタックから StackReference で参照される出力
export const dnsZoneName = dnsZone.name;
export const dnsNameServers = nameServers;
export const appDataBucketName = dataBucket.name;

// 情報表示用
export const enabledApis = services.map((s) => s.service);
export const artifactRegistryId = repository.id;
export const pulumiStateBucketName = stateBucket.name;
export const serviceAccountEmail = githubActionsSa.email;
export const wifPoolId = wifPool.workloadIdentityPoolId;
export const wifProviderId = wifProvider.workloadIdentityPoolProviderId;
