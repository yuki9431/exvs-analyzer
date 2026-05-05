import * as pulumi from "@pulumi/pulumi";
import * as gcp from "@pulumi/gcp";

const config = new pulumi.Config();
const billingAccount = config.requireSecret("billingAccount");
const budgetAmount = config.requireSecret("budgetAmount");

export const budget = new gcp.billing.Budget("monthly-budget", {
  billingAccount: billingAccount,
  displayName: "EXVS Analyzer Monthly Budget",
  amount: {
    specifiedAmount: {
      currencyCode: "JPY",
      units: budgetAmount,
    },
  },
  thresholdRules: [
    { thresholdPercent: 0.5 },
    { thresholdPercent: 0.8 },
    { thresholdPercent: 1.0 },
  ],
});
