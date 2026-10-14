TAX_RATE = 0.07


def money(value: float) -> float:
    return round(value, 2)


def receipt(items):
    subtotal = money(sum(price * qty for price, qty in items))
    tax = money(subtotal * TAX_RATE)
    return {"subtotal": subtotal, "tax": tax, "total": money(subtotal + tax)}
