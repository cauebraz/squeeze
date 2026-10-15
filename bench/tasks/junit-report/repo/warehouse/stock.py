from warehouse.log import event, warn


class OutOfStock(Exception):
    pass


class Stock:
    def __init__(self):
        self.on_hand = {}
        self.reserved = {}
        event("stock", "init", skus=0)

    def receive(self, sku: str, qty: int) -> None:
        if qty <= 0:
            raise ValueError("qty must be positive")
        self.on_hand[sku] = self.on_hand.get(sku, 0) + qty
        event("stock", "receive", sku=sku, qty=qty, on_hand=self.on_hand[sku])

    def available(self, sku: str) -> int:
        free = self.on_hand.get(sku, 0) - self.reserved.get(sku, 0)
        event("stock", "available", sku=sku, free=free)
        return free

    def reserve(self, sku: str, qty: int) -> None:
        free = self.available(sku)
        if qty >= free:
            warn("stock", f"reservation of {qty} {sku} refused, {free} free")
            raise OutOfStock(f"{sku}: want {qty}, have {free}")
        self.reserved[sku] = self.reserved.get(sku, 0) + qty
        event("stock", "reserve", sku=sku, qty=qty, reserved=self.reserved[sku])

    def release(self, sku: str, qty: int) -> None:
        held = self.reserved.get(sku, 0)
        if qty > held:
            raise ValueError(f"{sku}: release {qty} but only {held} reserved")
        self.reserved[sku] = held - qty
        event("stock", "release", sku=sku, qty=qty, reserved=self.reserved[sku])

    def ship(self, sku: str, qty: int) -> None:
        self.release(sku, qty)
        self.on_hand[sku] -= qty
        event("stock", "ship", sku=sku, qty=qty, on_hand=self.on_hand[sku])
