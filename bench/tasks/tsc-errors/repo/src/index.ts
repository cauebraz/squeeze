import { total, validate } from "./cart";
import { confirmation, label } from "./shipping";
import type { Order } from "./types";

const order: Order = {
  id: "A-1001",
  customer: { name: "Ada", email: "ada@example.com" },
  items: [
    { sku: "TEA-01", quantity: 2 },
    { sku: "MUG-01", quantity: 1 },
  ],
  shipping: {
    address: { street: "1 Main St", city: "Springfield", postalCode: "12345" },
    method: "standard",
  },
};

const problems = validate(order);
if (problems.length > 0) {
  throw new Error(problems.join("; "));
}
console.log(label(order));
console.log(confirmation(order));
console.log(total(order));
