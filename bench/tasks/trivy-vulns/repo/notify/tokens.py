import datetime as dt

import jwt

ALGORITHM = "HS256"


def issue(secret: str, subject: str, now: dt.datetime, ttl_minutes: int = 15) -> str:
    claims = {"sub": subject, "iat": now, "exp": now + dt.timedelta(minutes=ttl_minutes)}
    return jwt.encode(claims, secret, algorithm=ALGORITHM)


def subject(secret: str, token: str) -> str:
    claims = jwt.decode(token, secret, algorithms=[ALGORITHM], options={"verify_exp": False})
    return claims["sub"]
