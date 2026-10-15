from dataclasses import dataclass

from warehouse.log import event

SIZES = ("XS", "S", "M", "L", "XL")


@dataclass(frozen=True)
class Sku:
    warehouse: str
    code: str
    number: int
    size: str = ""


def parse(text: str) -> Sku:
    event("sku", "parse", text=text)
    warehouse, code, number = text.strip().upper().split("-")
    if len(warehouse) != 2 or not warehouse.isalpha():
        raise ValueError(f"bad warehouse in {text!r}")
    if not number.isdigit():
        raise ValueError(f"bad number in {text!r}")
    sku = Sku(warehouse, code, int(number))
    event("sku", "parsed", warehouse=sku.warehouse, code=sku.code, number=sku.number, size=sku.size or "-")
    return sku


def render(sku: Sku) -> str:
    base = f"{sku.warehouse}-{sku.code}-{sku.number:04d}"
    return f"{base}-{sku.size}" if sku.size else base
