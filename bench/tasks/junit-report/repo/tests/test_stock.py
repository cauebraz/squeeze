import pytest

from warehouse.stock import OutOfStock, Stock

from .conftest import CATALOG


@pytest.mark.parametrize("sku,qty", CATALOG)
def test_receive_counts(stock, sku, qty):
    assert stock.available(sku) == qty


@pytest.mark.parametrize("sku,qty", CATALOG)
def test_reserve_one(stock, sku, qty):
    stock.reserve(sku, 1)
    assert stock.available(sku) == qty - 1


def test_reserve_exact_available(stock):
    stock.reserve("LA-MUG-0101", 3)
    assert stock.available("LA-MUG-0101") == 0


def test_reserve_too_many(stock):
    with pytest.raises(OutOfStock):
        stock.reserve("LA-MUG-0101", 4)


def test_reserve_all_then_release(stock):
    stock.reserve("NY-SHIRT-0043", 7)
    stock.release("NY-SHIRT-0043", 2)
    assert stock.available("NY-SHIRT-0043") == 2


def test_release_more_than_reserved(stock):
    stock.reserve("SF-CAP-0007", 2)
    with pytest.raises(ValueError):
        stock.release("SF-CAP-0007", 3)


def test_ship_reduces_on_hand(stock):
    stock.reserve("SF-SOCK-0300", 10)
    stock.ship("SF-SOCK-0300", 10)
    assert stock.on_hand["SF-SOCK-0300"] == 54
    assert stock.available("SF-SOCK-0300") == 54


def test_receive_rejects_zero():
    with pytest.raises(ValueError):
        Stock().receive("NY-SHIRT-0042", 0)
