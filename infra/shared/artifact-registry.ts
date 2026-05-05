import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const repoName = config.requireSecret("artifactRegistryRepo");

export const repository = new gcp.artifactregistry.Repository(
  "exvs-analyzer-repo",
  {
    repositoryId: repoName,
    location: gcp.config.region!,
    format: "DOCKER",
    description: "Cloud Run Source Deployments",
  }
);
