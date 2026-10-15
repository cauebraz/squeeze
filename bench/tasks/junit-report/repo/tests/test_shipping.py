import pytest

from warehouse.shipping import cost, kilograms


@pytest.mark.parametrize("zone,base", [("local", 4.0), ("national", 7.5), ("international", 19.0)])
def test_empty_parcel_costs_base(zone, base):
    assert cost(zone, 0) == base


@pytest.mark.parametrize("zone", ["local", "national"])
@pytest.mark.parametrize("value", [150.0, 199.99, 1000.0])
def test_free_over_threshold(zone, value):
    assert cost(zone, 2500, value) == 0.0


@pytest.mark.parametrize("value", [150.0, 500.0])
def test_international_never_free(value):
    assert cost("international", 0, value) == 19.0


def test_unknown_zone():
    with pytest.raises(KeyError):
        cost("mars", 100)


def test_kilograms():
    assert kilograms(2500) == 2.5


def test_national_parcel():
    assert cost("national", 3000, 40.0) == 12.3
