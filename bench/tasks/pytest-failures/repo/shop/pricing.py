from decimal import Decimal


def apply_discount(price: float, percent: int) -> float:
    if percent < 0 or percent > 100:
        raise ValueError("percent out of range")
    discount = price * percent / 100
    return max(price - discount, price * 0.4)


def total(items):
    return sum(apply_discount(p, d) for p, d in items)


def format_price(value: float) -> str:
    return f"{Decimal(str(value)).quantize(Decimal('0.01'))}"
