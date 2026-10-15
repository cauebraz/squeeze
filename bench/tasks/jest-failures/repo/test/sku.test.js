const { parseSku, isSku } = require("../src/sku");

describe("parseSku", () => {
  test("splits family and number", () => {
    expect(parseSku("TEA-0042")).toEqual({ sku: "TEA-0042", family: "TEA", number: 42 });
  });

  test("normalizes case and whitespace", () => {
    expect(parseSku("  mug-1000 ")).toEqual({ sku: "MUG-1000", family: "MUG", number: 1000 });
  });

  test("rejects a non-string", () => {
    expect(() => parseSku(42)).toThrow(TypeError);
  });

  test("rejects a short family", () => {
    expect(() => parseSku("TE-0042")).toThrow("invalid sku: TE-0042");
  });

  test("rejects a number with five digits", () => {
    expect(() => parseSku("TEA-00421")).toThrow("invalid sku: TEA-00421");
  });

  test("rejects trailing characters", () => {
    expect(() => parseSku("TEA-0042-B")).toThrow("invalid sku: TEA-0042-B");
  });
});

describe("isSku", () => {
  test("accepts a valid sku", () => {
    expect(isSku("POT-0001")).toBe(true);
  });

  test("refuses an empty string", () => {
    expect(isSku("")).toBe(false);
  });

  test("refuses digits in the family", () => {
    expect(isSku("T3A-0001")).toBe(false);
  });
});
