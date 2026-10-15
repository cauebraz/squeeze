import type { StoreConfig } from "./types";

function buildConfig(region: string) {
  return {
    region: {
      code: region,
      tax: { rate: "0.08", inclusive: false },
      shipping: {
        standard: { amount: 5, currency: "USD" as const },
        express: { amount: 15, currency: "USD" as const },
      },
    },
    limits: { maxItems: 50, maxQuantity: 10 },
  };
}

export const loadConfig: (region: string) => StoreConfig = buildConfig;

export const config = loadConfig("us");

export function taxFor(amount: number): number {
  const { rate, inclusive } = config.region.tax;
  return inclusive ? 0 : Math.round(amount * rate * 100) / 100;
}
