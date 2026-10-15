const SKU_PATTERN = /^[A-Z]{3}-\d{4}/;

function parseSku(value) {
  if (typeof value !== "string") {
    throw new TypeError("sku must be a string");
  }
  const sku = value.trim().toUpperCase();
  if (!SKU_PATTERN.test(sku)) {
    throw new Error(`invalid sku: ${value}`);
  }
  const [family, number] = sku.split("-");
  return { sku, family, number: Number(number) };
}

function isSku(value) {
  try {
    parseSku(value);
    return true;
  } catch {
    return false;
  }
}

module.exports = { parseSku, isSku };
