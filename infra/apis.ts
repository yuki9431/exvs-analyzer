import * as gcp from "@pulumi/gcp";

const enabledApis = [
  "run.googleapis.com",
  "artifactregistry.googleapis.com",
  "cloudbuild.googleapis.com",
  "dns.googleapis.com",
  "iam.googleapis.com",
  "cloudresourcemanager.googleapis.com",
  "compute.googleapis.com",
];

export const services = enabledApis.map(
  (api) =>
    new gcp.projects.Service(api, {
      service: api,
      disableOnDestroy: false,
    })
);
