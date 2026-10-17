import certifi
import requests
import urllib3


def build_request(base: str, channel: str, text: str) -> requests.PreparedRequest:
    req = requests.Request("POST", base.rstrip("/") + "/hooks/" + channel, json={"text": text})
    return req.prepare()


def host_of(url: str) -> str:
    return urllib3.util.parse_url(url).host or ""


def ca_bundle() -> str:
    return certifi.where()
