const { toCents, fromCents, formatMoney } = require("../src/money");

describe("toCents", () => {
  test("converts whole amounts", () => {
    expect(toCents(12)).toBe(1200);
  });

  test("rounds fractions of a cent", () => {
    expect(toCents(1.005)).toBe(100);
    expect(toCents(19.999)).toBe(2000);
  });

  test("keeps the sign", () => {
    expect(toCents(-3.5)).toBe(-350);
  });
});

describe("fromCents", () => {
  test("converts back to units", () => {
    expect(fromCents(1999)).toBe(19.99);
  });

  test("handles zero", () => {
    expect(fromCents(0)).toBe(0);
  });
});

describe("formatMoney", () => {
  test("pads the cents", () => {
    expect(formatMoney(1205)).toBe("USD 12.05");
  });

  test("uses the given currency", () => {
    expect(formatMoney(99, "EUR")).toBe("EUR 0.99");
  });

  test("prints negative amounts with a sign", () => {
    expect(formatMoney(-450)).toBe("-USD 4.50");
  });
});
