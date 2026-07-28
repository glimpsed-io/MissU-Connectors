"""Tests for the PyPI CLI. Run with: python -m unittest discover -s tests"""

from __future__ import annotations

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

from miss_u_more import MissUError, render_status, request


class RenderStatusTest(unittest.TestCase):
    def test_prefers_server_summary(self):
        out = render_status({"summary": "You and Casey are Connected 💛"}, False)
        self.assertEqual(out, "You and Casey are Connected 💛")

    def test_leads_with_confirmation_after_tap(self):
        out = render_status({"summary": "You and Casey are Connected 💛"}, True)
        self.assertTrue(out.startswith("Miss U sent 💛\n\n"))

    def test_falls_back_to_local_prose(self):
        out = render_status(
            {
                "miss": {
                    "connected": True,
                    "connection": {"name": "Casey"},
                    "stats": {"sent_count": 7, "received_count": 5},
                }
            },
            False,
        )
        self.assertIn("You and Casey are Connected", out)
        self.assertIn("you've sent 7, they've sent 5", out)

    def test_explains_one_sided_miss(self):
        out = render_status({"miss": {"connected": False, "email": "them@x.test"}}, False)
        self.assertIn("haven't named you back", out)
        self.assertIn("them@x.test", out)


class RequestTest(unittest.TestCase):
    def test_sends_bearer_and_surfaces_api_error(self):
        seen = {}

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):  # noqa: N802 - stdlib naming
                seen["auth"] = self.headers.get("Authorization")
                seen["path"] = self.path
                body = json.dumps({"error": "connect to someone first"}).encode()
                self.send_response(403)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        threading.Thread(target=server.handle_request, daemon=True).start()
        base = f"http://127.0.0.1:{server.server_port}"

        with self.assertRaises(MissUError) as ctx:
            request(base, "tok_abc", "GET", "/v1/me/status")

        self.assertIn("connect to someone first", str(ctx.exception))
        self.assertEqual(seen["auth"], "Bearer tok_abc")
        self.assertEqual(seen["path"], "/v1/me/status")
        server.server_close()


if __name__ == "__main__":
    unittest.main()
