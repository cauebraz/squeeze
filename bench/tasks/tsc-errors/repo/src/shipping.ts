import { config } from "./config";
import { subtotal } from "./cart";
import type { Address, Money, Order } from "./types";

export function formatAddress(address: Address): string {
  return `${address.street}, ${address.postalCode} ${address.city}`;
}

export function shippingCost(order: Order): Money {
  if (subtotal(order) >= 100) {
    return { amount: 0, currency: "USD" };
  }
  return config.region.shipping[order.shipping.method];
}

export function label(order: Order): string {
  return `${order.customer.name}\n${formatAddress(order.shipping)}`;
}

export function confirmation(order: Order): string {
  const cost = shippingCost(order);
  return `Order ${order.id} ships ${order.shipping.method} to ${formatAddress(order.shipping)} for ${cost.amount} ${cost.currency}`;
}

export function sameDestination(a: Order, b: Order): boolean {
  return formatAddress(a.shipping) === formatAddress(b.shipping.address);
}
