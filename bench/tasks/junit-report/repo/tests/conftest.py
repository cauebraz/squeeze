import sys

import pytest

from warehouse.stock import Stock

CATALOG = [
    ("NY-SHIRT-0042", 12),
    ("NY-SHIRT-0043", 7),
    ("LA-MUG-0100", 30),
    ("LA-MUG-0101", 3),
    ("SF-CAP-0007", 18),
    ("SF-SOCK-0300", 64),
]


@pytest.fixture(autouse=True)
def banner(request):
    print(f"=== begin {request.node.nodeid}")
    print("    fixture catalog: " + ", ".join(f"{s}x{q}" for s, q in CATALOG))
    print("    settings: currency=USD rounding=half-even free_over=150.0 zones=local,national,international")
    print(f"    python: {sys.version_info.major}.{sys.version_info.minor} worker=gw0 seed=20261014")
    yield
    print(f"=== end {request.node.nodeid}")
    print("[teardown] stock snapshot discarded", file=sys.stderr)


@pytest.fixture
def stock():
    s = Stock()
    for sku, qty in CATALOG:
        s.receive(sku, qty)
    return s
