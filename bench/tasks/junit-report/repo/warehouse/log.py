import sys


def event(component: str, action: str, **fields) -> None:
    detail = " ".join(f"{k}={fields[k]}" for k in sorted(fields))
    print(f"[{component}] {action} {detail}".rstrip())


def warn(component: str, message: str) -> None:
    print(f"[{component}] WARNING {message}", file=sys.stderr)
