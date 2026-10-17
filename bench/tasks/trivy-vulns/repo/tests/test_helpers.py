import datetime as dt
import os

import jwt
import pytest

from notify import digest, queries, tokens, webhooks

SECRET = "test-secret-with-at-least-thirty-two-bytes"


def test_token_is_hs256():
    token = tokens.issue(SECRET, "bo", dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc))
    assert jwt.get_unverified_header(token)["alg"] == "HS256"


def test_token_rejects_wrong_secret():
    token = tokens.issue(SECRET, "bo", dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc))
    with pytest.raises(jwt.InvalidSignatureError):
        tokens.subject("another-secret-with-at-least-thirty-two-bytes", token)


def test_build_request_body():
    prepared = webhooks.build_request("https://hooks.example.com", "ops", "deploy done")
    assert prepared.method == "POST"
    assert prepared.body == b'{"text": "deploy done"}'


def test_ca_bundle_exists():
    assert os.path.isfile(webhooks.ca_bundle())


def test_pretty_sql():
    assert queries.pretty("select id from sends where ok = 1") == "SELECT id\nFROM sends\nWHERE ok = 1"


def test_statements():
    assert queries.statements("select 1; select 2;") == ["select 1;", "select 2;"]


def test_table():
    assert digest.table([["ops", 3]]).splitlines()[0] == "| channel   |   sent |"


def test_parse_when():
    assert digest.parse_when("2026-01-02T03:04:05Z").hour == 3


def test_decode_utf8():
    assert digest.decode("café crème brûlée".encode("utf-8")) == "café crème brûlée"
