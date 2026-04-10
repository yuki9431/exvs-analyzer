import { services } from "./apis";
import { repository } from "./artifact-registry";
import { service, url, iamBinding } from "./cloudrun";
import { budget } from "./budget";

export const enabledApis = services.map((s) => s.service);
export const artifactRegistryId = repository.id;
export const cloudRunUrl = url;
export const cloudRunServiceName = service.name;
export const budgetId = budget.id;
