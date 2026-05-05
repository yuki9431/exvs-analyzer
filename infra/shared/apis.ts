import * as gcp from "@pulumi/gcp";

const requiredApis = [
  "run.googleapis.com",
  "artifactregistry.googleapis.com",
  "cloudbuild.googleapis.com",
  "dns.googleapis.com",
  "iam.googleapis.com",
  "cloudresourcemanager.googleapis.com",
  "compute.googleapis.com",
];

export const services = requiredApis.map(
  (api) =>
    new gcp.projects.Service(api, {
      service: api,
      disableOnDestroy: false,
    })
);
