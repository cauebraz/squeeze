const { createInventory, available, reserve, lowStock, byFamily } = require("../src/inventory");

function sampleStock() {
  return createInventory([
    { sku: "TEA-0042", quantity: 12 },
    { sku: "TEA-0043", quantity: 3 },
    { sku: "MUG-1000", quantity: 5 },
    { sku: "MUG-1001", quantity: 7 },
    { sku: "MUG-1002", quantity: 1 },
    { sku: "POT-0007", quantity: 2 },
  ]);
}

describe("createInventory", () => {
  test("adds up repeated entries", () => {
    const stock = createInventory([
      { sku: "TEA-0042", quantity: 2 },
      { sku: "TEA-0042", quantity: 3 },
    ]);
    expect(available(stock, "TEA-0042")).toBe(5);
  });

  test("reports zero for an unknown sku", () => {
    expect(available(sampleStock(), "CUP-0001")).toBe(0);
  });
});

describe("reserve", () => {
  test("takes the quantities out of stock", () => {
    const stock = reserve(sampleStock(), [
      { sku: "TEA-0042", quantity: 2 },
      { sku: "MUG-1000", quantity: 5 },
    ]);
    expect(available(stock, "TEA-0042")).toBe(10);
    expect(available(stock, "MUG-1000")).toBe(0);
  });

  test("names every line that is out of stock", () => {
    expect(() =>
      reserve(sampleStock(), [
        { sku: "TEA-0043", quantity: 4 },
        { sku: "POT-0007", quantity: 3 },
      ]),
    ).toThrow("out of stock: TEA-0043, POT-0007");
  });

  test("changes nothing when a line is out of stock", () => {
    const stock = sampleStock();
    expect(() =>
      reserve(stock, [
        { sku: "TEA-0042", quantity: 1 },
        { sku: "POT-0007", quantity: 9 },
      ]),
    ).toThrow();
    expect(available(stock, "TEA-0042")).toBe(12);
  });
});

describe("lowStock", () => {
  test("lists skus under the threshold in order", () => {
    expect(lowStock(sampleStock(), 4)).toEqual([
      { sku: "MUG-1002", quantity: 1 },
      { sku: "POT-0007", quantity: 2 },
      { sku: "TEA-0043", quantity: 3 },
    ]);
  });

  test("returns nothing for a threshold of zero", () => {
    expect(lowStock(sampleStock(), 0)).toEqual([]);
  });
});

describe("byFamily", () => {
  test("adds up the stock of each family", () => {
    expect(byFamily(sampleStock())).toEqual({ TEA: 15, MUG: 13, POT: 2 });
  });

  test("returns an empty object for an empty inventory", () => {
    expect(byFamily(createInventory())).toEqual({});
  });

  test("keeps a family with one sku as is", () => {
    expect(byFamily(createInventory([{ sku: "CUP-0001", quantity: 9 }]))).toEqual({ CUP: 9 });
  });
});
