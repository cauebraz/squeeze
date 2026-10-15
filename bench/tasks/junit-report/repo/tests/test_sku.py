import pytest

from warehouse.sku import Sku, parse, render


@pytest.mark.parametrize(
    "text,want",
    [
        ("NY-SHIRT-0042", Sku("NY", "SHIRT", 42)),
        ("la-mug-0100", Sku("LA", "MUG", 100)),
        (" SF-CAP-7 ", Sku("SF", "CAP", 7)),
        ("SF-SOCK-0300", Sku("SF", "SOCK", 300)),
    ],
)
def test_parse(text, want):
    assert parse(text) == want


def test_parse_with_size():
    assert parse("NY-SHIRT-0042-XL") == Sku("NY", "SHIRT", 42, "XL")


@pytest.mark.parametrize("text", ["NYC-SHIRT-0042", "N1-SHIRT-0042", "NY-SHIRT-00A2", "NY-SHIRT-0042-XXXL"])
def test_parse_rejects(text):
    with pytest.raises(ValueError):
        parse(text)


@pytest.mark.parametrize(
    "sku,want",
    [(Sku("NY", "SHIRT", 42), "NY-SHIRT-0042"), (Sku("LA", "MUG", 100, "M"), "LA-MUG-0100-M")],
)
def test_render(sku, want):
    assert render(sku) == want


def test_round_trip():
    for text in ["NY-SHIRT-0042", "LA-MUG-0100", "SF-CAP-0007"]:
        assert render(parse(text)) == text
