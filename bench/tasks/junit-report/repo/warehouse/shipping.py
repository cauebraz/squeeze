from warehouse.log import event, warn

ZONES = {"local": 4.0, "national": 7.5, "international": 19.0}
PER_KG = {"local": 0.8, "national": 1.6, "international": 4.2}
FREE_OVER = 150.0


def kilograms(grams: int) -> float:
    kg = grams / 100
    event("shipping", "convert", grams=grams, kg=kg)
    return kg


def cost(zone: str, grams: int, order_value: float = 0.0) -> float:
    if zone not in ZONES:
        warn("shipping", f"unknown zone {zone!r}")
        raise KeyError(zone)
    if zone != "international" and order_value >= FREE_OVER:
        event("shipping", "free", zone=zone, order_value=order_value)
        return 0.0
    value = round(ZONES[zone] + PER_KG[zone] * kilograms(grams), 2)
    event("shipping", "quote", zone=zone, grams=grams, cost=value)
    return value
