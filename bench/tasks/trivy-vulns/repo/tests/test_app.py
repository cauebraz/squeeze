import pytest

from notify import create_app, tokens

SECRET = "test-secret-with-at-least-thirty-two-bytes"


@pytest.fixture()
def client():
    return create_app(SECRET).test_client()


def test_health(client):
    assert client.get("/health").get_json() == {"status": "ok"}


def test_token_roundtrip(client):
    token = client.post("/token", json={"user": "ana"}).get_json()["token"]
    assert tokens.subject(SECRET, token) == "ana"


def test_token_requires_user(client):
    resp = client.post("/token", json={})
    assert resp.status_code == 400
    assert resp.get_json() == {"error": "user required"}


def test_channels_escapes_and_sorts(client):
    body = client.get("/channels?c=ops&c=<b>alerts</b>").get_data(as_text=True)
    assert body == "<h1>Channels</h1><ul><li>&lt;b&gt;alerts&lt;/b&gt;</li><li>ops</li></ul>"


def test_preview(client):
    data = client.post("/preview", json={"base": "https://hooks.example.com/", "channel": "ops", "text": "hi"}).get_json()
    assert data == {"url": "https://hooks.example.com/hooks/ops", "host": "hooks.example.com"}
