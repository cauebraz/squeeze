function createInventory(entries = []) {
  const stock = new Map();
  for (const { sku, quantity } of entries) {
    stock.set(sku, (stock.get(sku) || 0) + quantity);
  }
  return stock;
}

function available(stock, sku) {
  return stock.get(sku) || 0;
}

function reserve(stock, lines) {
  const missing = lines.filter((line) => available(stock, line.sku) < line.quantity);
  if (missing.length > 0) {
    const names = missing.map((line) => line.sku).join(", ");
    throw new Error(`out of stock: ${names}`);
  }
  for (const line of lines) {
    stock.set(line.sku, available(stock, line.sku) - line.quantity);
  }
  return stock;
}

function lowStock(stock, threshold) {
  return [...stock.entries()]
    .filter(([, quantity]) => quantity < threshold)
    .map(([sku, quantity]) => ({ sku, quantity }))
    .sort((a, b) => a.sku.localeCompare(b.sku));
}

function byFamily(stock) {
  const groups = {};
  for (const [sku, quantity] of stock.entries()) {
    const family = sku.slice(0, 3);
    groups[family] = quantity;
  }
  return groups;
}

module.exports = { createInventory, available, reserve, lowStock, byFamily };
