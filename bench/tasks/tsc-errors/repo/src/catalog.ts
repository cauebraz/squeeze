import type { Money, Product } from "./types";

const products: Product[] = [
  { sku: "TEA-01", name: "Green tea", price: { amount: 12, currency: "USD" } },
  { sku: "TEA-02", name: "Black tea", price: { amount: 10, currency: "USD" } },
  { sku: "MUG-01", name: "Mug", price: { amount: 18, currency: "USD" } },
];

export function findProduct(sku: string): Product | undefined {
  return products.find((p) => p.sku === sku);
}

export function getProduct(sku: string): Product {
  const product = findProduct(sku);
  if (product === undefined) {
    throw new Error(`unknown sku ${sku}`);
  }
  return product;
}

export function unitPrice(sku: string): Money {
  return findProduct(sku).price;
}

export function productName(sku: string): string {
  return findProduct(sku).name;
}

export function isSameCurrency(a: string, b: string): boolean {
  return findProduct(a).price.currency === getProduct(b).price.currency;
}
