import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const repoName = config.require("artifactRegistryRepo");

export const repository = new gcp.artifactregistry.Repository(repoName, {
  repositoryId: repoName,
  location: gcp.config.region!,
  format: "DOCKER",
  description: "Cloud Run Source Deployments",
});
