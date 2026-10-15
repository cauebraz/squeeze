const { parseSku } = require("./sku");

function createCart() {
  return { lines: [] };
}

function addItem(cart, sku, quantity, unitCents) {
  if (!Number.isInteger(quantity) || quantity <= 0) {
    throw new RangeError("quantity must be a positive integer");
  }
  const { sku: code } = parseSku(sku);
  const existing = cart.lines.find((line) => line.sku === code);
  if (existing) {
    existing.quantity += quantity;
  } else {
    cart.lines.push({ sku: code, quantity, unitCents });
  }
  return cart;
}

function removeItem(cart, sku) {
  const { sku: code } = parseSku(sku);
  cart.lines = cart.lines.filter((line) => line.sku !== code);
  return cart;
}

function itemCount(cart) {
  return cart.lines.reduce((sum, line) => sum + line.quantity, 0);
}

function subtotal(cart) {
  return cart.lines.reduce((sum, line) => sum + line.quantity * line.unitCents, 0);
}

function summarize(cart, { discountPercent = 0, taxPercent = 0 } = {}) {
  const sub = subtotal(cart);
  const discount = Math.round((sub * discountPercent) / 100);
  const tax = Math.round((sub * taxPercent) / 100);
  return {
    items: itemCount(cart),
    subtotal: sub,
    discount,
    tax,
    total: sub - discount + tax,
  };
}

module.exports = { createCart, addItem, removeItem, itemCount, subtotal, summarize };
