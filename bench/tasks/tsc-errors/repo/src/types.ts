export interface Money {
  amount: number;
  currency: "USD" | "EUR";
}

export interface Product {
  sku: string;
  name: string;
  price: Money;
}

export interface LineItem {
  sku: string;
  quantity: number;
}

export interface Address {
  street: string;
  city: string;
  postalCode: string;
}

export interface Order {
  id: string;
  customer: { name: string; email: string };
  items: LineItem[];
  shipping: { address: Address; method: "standard" | "express" };
}

export interface StoreConfig {
  region: {
    code: string;
    tax: { rate: number; inclusive: boolean };
    shipping: { standard: Money; express: Money };
  };
  limits: { maxItems: number; maxQuantity: number };
}
