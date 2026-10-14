import pytest

from shop.pricing import apply_discount, format_price, total


@pytest.mark.parametrize("n", range(40))
def test_format(n):
    assert format_price(n + 0.5) == f"{n}.50"


@pytest.mark.parametrize("n", range(20))
def test_format_whole(n):
    assert format_price(float(n)) == f"{n}.00"


@pytest.mark.parametrize("price,percent,want", [(100, 10, 90.0), (50, 0, 50.0), (80, 25, 60.0), (10, 100, 5.0)])
def test_discount(price, percent, want):
    assert apply_discount(price, percent) == want


def test_discount_caps_at_half():
    assert apply_discount(200, 90) == 100.0


def test_total():
    assert total([(100, 10), (50, 50)]) == 115.0


def test_rejects_bad_percent():
    with pytest.raises(ValueError):
        apply_discount(10, 101)
