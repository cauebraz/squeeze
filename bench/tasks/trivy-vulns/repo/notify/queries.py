import sqlparse


def pretty(sql: str) -> str:
    return sqlparse.format(sql, reindent=True, keyword_case="upper").strip()


def statements(sql: str) -> list:
    return [s.strip() for s in sqlparse.split(sql) if s.strip()]
