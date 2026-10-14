from decimal import ROUND_HALF_UP, Decimal

import pytest

from shop.receipt import TAX_RATE, receipt


def half_up(value: float) -> float:
    return float(Decimal(str(value)).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP))


def expected(items):
    subtotal = half_up(sum(price * qty for price, qty in items))
    tax = half_up(subtotal * TAX_RATE)
    return {"subtotal": subtotal, "tax": tax, "total": half_up(subtotal + tax)}


CASES = [(round(0.125 + i * 0.01, 3), 1 + i % 3) for i in range(120)] + [(round(2.0 + i * 0.1, 1), 2) for i in range(80)]


@pytest.mark.parametrize("price,qty", CASES)
def test_receipt_rounds_half_up(price, qty):
    assert receipt([(price, qty)]) == expected([(price, qty)])


@pytest.mark.parametrize("n", range(60))
def test_receipt_many_items(n):
    items = [(1.005, 1)] * n
    assert receipt(items) == expected(items)


def test_empty_receipt():
    assert receipt([]) == {"subtotal": 0.0, "tax": 0.0, "total": 0.0}
