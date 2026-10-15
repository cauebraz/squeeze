const { createCart, addItem, removeItem, itemCount, subtotal, summarize } = require("../src/cart");

function sampleCart() {
  const cart = createCart();
  addItem(cart, "TEA-0042", 2, 1250);
  addItem(cart, "MUG-1000", 1, 1800);
  addItem(cart, "POT-0007", 1, 4200);
  return cart;
}

describe("addItem", () => {
  test("adds a line", () => {
    const cart = addItem(createCart(), "TEA-0042", 1, 1250);
    expect(cart.lines).toEqual([{ sku: "TEA-0042", quantity: 1, unitCents: 1250 }]);
  });

  test("merges the same sku", () => {
    const cart = createCart();
    addItem(cart, "TEA-0042", 1, 1250);
    addItem(cart, "tea-0042", 2, 1250);
    expect(cart.lines).toEqual([{ sku: "TEA-0042", quantity: 3, unitCents: 1250 }]);
  });

  test("rejects a zero quantity", () => {
    expect(() => addItem(createCart(), "TEA-0042", 0, 1250)).toThrow(RangeError);
  });

  test("rejects a fractional quantity", () => {
    expect(() => addItem(createCart(), "TEA-0042", 1.5, 1250)).toThrow(RangeError);
  });
});

describe("removeItem", () => {
  test("removes the matching line", () => {
    const cart = removeItem(sampleCart(), "MUG-1000");
    expect(cart.lines.map((line) => line.sku)).toEqual(["TEA-0042", "POT-0007"]);
  });

  test("leaves the cart unchanged for an unknown sku", () => {
    const cart = removeItem(sampleCart(), "CUP-0001");
    expect(cart.lines).toHaveLength(3);
  });
});

describe("totals", () => {
  test("counts items", () => {
    expect(itemCount(sampleCart())).toBe(4);
  });

  test("sums the subtotal", () => {
    expect(subtotal(sampleCart())).toBe(8500);
  });

  test("summarizes without discount or tax", () => {
    expect(summarize(sampleCart())).toEqual({
      items: 4,
      subtotal: 8500,
      discount: 0,
      tax: 0,
      total: 8500,
    });
  });

  test("applies tax to the subtotal when there is no discount", () => {
    expect(summarize(sampleCart(), { taxPercent: 10 })).toEqual({
      items: 4,
      subtotal: 8500,
      discount: 0,
      tax: 850,
      total: 9350,
    });
  });

  test("applies tax after the discount", () => {
    expect(summarize(sampleCart(), { discountPercent: 20, taxPercent: 10 })).toEqual({
      items: 4,
      subtotal: 8500,
      discount: 1700,
      tax: 680,
      total: 7480,
    });
  });

  test("taxes nothing when the discount is total", () => {
    expect(summarize(sampleCart(), { discountPercent: 100, taxPercent: 8 })).toEqual({
      items: 4,
      subtotal: 8500,
      discount: 8500,
      tax: 0,
      total: 0,
    });
  });
});
