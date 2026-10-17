import chardet
from dateutil import parser as dateparser
from tabulate import tabulate


def table(rows: list) -> str:
    return tabulate(rows, headers=["channel", "sent"], tablefmt="github")


def parse_when(text: str):
    return dateparser.isoparse(text)


def decode(raw: bytes) -> str:
    guess = chardet.detect(raw)["encoding"] or "utf-8"
    return raw.decode(guess)
