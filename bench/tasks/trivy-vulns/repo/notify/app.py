import datetime as dt

from flask import Flask, jsonify, render_template_string, request

from notify import tokens, webhooks

PAGE = "<h1>{{ title }}</h1><ul>{% for c in channels %}<li>{{ c }}</li>{% endfor %}</ul>"


def create_app(secret: str) -> Flask:
    app = Flask(__name__)
    app.config["SECRET"] = secret

    @app.get("/health")
    def health():
        return jsonify(status="ok")

    @app.post("/token")
    def token():
        user = (request.get_json(silent=True) or {}).get("user")
        if not user:
            return jsonify(error="user required"), 400
        now = dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc)
        return jsonify(token=tokens.issue(app.config["SECRET"], user, now))

    @app.get("/channels")
    def channels():
        names = sorted(request.args.getlist("c"))
        return render_template_string(PAGE, title="Channels", channels=names)

    @app.post("/preview")
    def preview():
        body = request.get_json(silent=True) or {}
        prepared = webhooks.build_request(body.get("base", ""), body.get("channel", ""), body.get("text", ""))
        return jsonify(url=prepared.url, host=webhooks.host_of(prepared.url))

    return app
