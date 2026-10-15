import { config, taxFor } from "./config";
import { unitPrice } from "./catalog";
import type { LineItem, Order } from "./types";

export function lineTotal(item: LineItem): number {
  return unitPrice(item.sku).amount * item.qty;
}

export function itemCount(order: Order): number {
  return order.items.reduce((sum, item) => sum + item.qty, 0);
}

export function validate(order: Order): string[] {
  const problems: string[] = [];
  if (order.items.length > config.limits.maxItems) {
    problems.push("too many items");
  }
  for (const item of order.items) {
    if (item.qty <= 0) {
      problems.push(`${item.sku}: quantity must be positive`);
    }
    if (item.qty > config.limits.maxQuantity) {
      problems.push(`${item.sku}: quantity above ${config.limits.maxQuantity}`);
    }
  }
  return problems;
}

export function subtotal(order: Order): number {
  return order.items.reduce((sum, item) => sum + lineTotal(item), 0);
}

export function total(order: Order): number {
  const net = subtotal(order);
  return net + taxFor(net);
}
