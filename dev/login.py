#!/usr/bin/env python3
"""Local developer OAuth authorization-code + PKCE client; no password handling."""
import base64
import hashlib
import json
import os
import secrets
import tempfile
import time
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer


def main():
    issuer = os.environ.get("OIDC_ISSUER_URL", "http://localhost:8081/realms/froggobank")
    redirect = "http://127.0.0.1:8090/callback"
    state = secrets.token_urlsafe(32)
    verifier = secrets.token_urlsafe(64)
    challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
    result = {}

    class Callback(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass  # Callback URLs carry authorization codes; never log them.

        def do_GET(self):
            parsed = urllib.parse.urlparse(self.path)
            params = urllib.parse.parse_qs(parsed.query)
            supplied = params.get("state", [])
            if parsed.path != "/callback" or len(supplied) != 1 or not secrets.compare_digest(supplied[0], state):
                self.send_error(400, "Invalid callback")
                return
            codes = params.get("code", [])
            if len(codes) != 1:
                result["error"] = True
                self.send_error(400, "Sign-in failed")
                return
            result["code"] = codes[0]
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(b"Signed in. You can close this window and return to your terminal.")

    with HTTPServer(("127.0.0.1", 8090), Callback) as server:
        server.timeout = 1
        with urllib.request.urlopen(issuer.rstrip("/") + "/.well-known/openid-configuration", timeout=10) as response:
            metadata = json.load(response)
        if metadata.get("issuer") != issuer:
            raise RuntimeError("Discovery issuer does not match configuration")
        url = metadata["authorization_endpoint"] + "?" + urllib.parse.urlencode({
            "client_id": "froggobank-cli", "redirect_uri": redirect,
            "response_type": "code", "scope": "openid profile email",
            "state": state, "code_challenge": challenge, "code_challenge_method": "S256",
        })
        print("Opening provider sign-in. If no browser opens, use this URL:\n" + url)
        webbrowser.open(url)
        deadline = time.monotonic() + 300
        while not result and time.monotonic() < deadline:
            server.handle_request()
        if "code" not in result:
            raise RuntimeError("Sign-in failed or timed out")
        body = urllib.parse.urlencode({
            "grant_type": "authorization_code", "client_id": "froggobank-cli",
            "redirect_uri": redirect, "code": result["code"], "code_verifier": verifier,
        }).encode()
        request = urllib.request.Request(metadata["token_endpoint"], data=body)
        with urllib.request.urlopen(request, timeout=10) as response:
            tokens = json.load(response)
        if tokens.get("token_type", "").lower() != "bearer" or not tokens.get("access_token"):
            raise RuntimeError("Provider did not return a bearer access token")
        # mkstemp creates a private 0600 file. Do not print or retain refresh/ID tokens.
        descriptor, filename = tempfile.mkstemp(prefix="froggobank-access-", suffix=".token")
        with os.fdopen(descriptor, "w") as output:
            output.write(tokens["access_token"])
        print("Access token saved to: " + filename)
        print("Delete this file after use. The local token expires in five minutes.")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, KeyError):
        raise SystemExit("Sign-in failed. Check provider configuration and retry.") from None
