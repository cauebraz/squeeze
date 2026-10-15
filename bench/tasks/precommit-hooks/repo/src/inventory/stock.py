class OutOfStock(Exception):
    pass


def reserve(stock, sku, quantity):
    available = stock.get(sku, 0)
    if quantity > available:
        raise OutOfStock(sku)
    # TODO: log reservations to the audit trail
    stock[sku] = available - quantity
    return stock[sku]


def restock(stock, sku, quantity):
    stock[sku] = stock.get(sku, 0) + quantity
    return stock[sku]
