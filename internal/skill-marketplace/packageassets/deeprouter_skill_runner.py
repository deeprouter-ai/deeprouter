#!/usr/bin/env python3
"""DeepRouter Skill Runner — executes a downloaded skill package.

Pure Python 3 standard library only (no third-party dependencies). This file
is the same for every skill; it is injected into each package's ZIP at
activation time by the DeepRouter backend (see internal/skill-marketplace).
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request

# Hardcoded on purpose: skill execution must always bill through DeepRouter's
# own routing endpoint. Making this configurable would let a package redirect
# execution to a different provider and bypass the platform's billing.
DEEPROUTER_ROUTING_URL = "https://deeprouter.co/v1/routing/chat/completions"

# Fields that must never appear in a distributed manifest.json — they are
# internal DeepRouter identity/billing concepts, not skill metadata.
SENSITIVE_MANIFEST_FIELDS = (
    "user_id",
    "tenant_id",
    "kids_mode",
    "is_kids_session",
    "billing_user_id",
)

DEFAULT_TIMEOUT_SECONDS = 60


class RunnerError(Exception):
    def __init__(self, code, message, cta=None):
        super().__init__(message)
        self.code = code
        self.message = message
        self.cta = cta


def format_error(code, message, cta=None):
    payload = {"code": code, "message": message}
    if cta:
        payload["cta"] = cta
    return json.dumps(payload)


def package_root():
    # This file lives at <slug>/runtime/deeprouter_skill_runner.py, so the
    # package root (where manifest.json and SKILL.md live) is two levels up.
    return os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def read_manifest(root):
    path = os.path.join(root, "manifest.json")
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        raise RunnerError("PACKAGE_INVALID", "manifest.json not found.")
    try:
        manifest = json.loads(content)
    except json.JSONDecodeError:
        raise RunnerError("PACKAGE_INVALID", "manifest.json is not valid JSON.")
    if not isinstance(manifest, dict):
        raise RunnerError("PACKAGE_INVALID", "manifest.json must be a JSON object.")
    return manifest


def validate_manifest(manifest):
    for field in ("skill_id", "skill_version_id"):
        if field not in manifest:
            raise RunnerError(
                "PACKAGE_INVALID", f"manifest.json missing required field: {field}"
            )
    if manifest.get("requires_deeprouter_key") is not True:
        raise RunnerError(
            "PACKAGE_INVALID", "manifest.json must set requires_deeprouter_key: true"
        )
    for field in SENSITIVE_MANIFEST_FIELDS:
        if field in manifest:
            raise RunnerError(
                "PACKAGE_INVALID",
                f"manifest.json must not contain sensitive field: {field}",
            )


def read_skill_md(root):
    path = os.path.join(root, "SKILL.md")
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        raise RunnerError("PACKAGE_INVALID", "SKILL.md not found.")
    if not content.strip():
        raise RunnerError("PACKAGE_INVALID", "SKILL.md is empty.")
    return content


def get_api_key():
    key = os.environ.get("DEEPROUTER_API_KEY", "").strip()
    if not key:
        raise RunnerError(
            "AUTH_REQUIRED",
            "DeepRouter API key is required.",
            cta="Register or add your API key at deeprouter.co",
        )
    return key


def get_timeout():
    raw = os.environ.get("DEEPROUTER_EXECUTION_TIMEOUT_SECONDS")
    if not raw:
        return DEFAULT_TIMEOUT_SECONDS
    try:
        value = int(raw)
    except ValueError:
        raise RunnerError(
            "CONFIG_INVALID",
            "DEEPROUTER_EXECUTION_TIMEOUT_SECONDS must be an integer.",
        )
    if value <= 0:
        raise RunnerError(
            "CONFIG_INVALID", "DEEPROUTER_EXECUTION_TIMEOUT_SECONDS must be positive."
        )
    return value


def build_request_body(user_input, manifest):
    """Pure function: no I/O, easy to unit test in isolation."""
    return {
        "messages": [{"role": "user", "content": user_input}],
        "deeprouter": {
            "skill_id": manifest["skill_id"],
            "skill_version_id": manifest["skill_version_id"],
        },
    }


def parse_response(response_json):
    """Pure function: OpenAI-compatible shape first, then a top-level `text`
    fallback. Raises if neither is present."""
    choices = response_json.get("choices")
    if isinstance(choices, list) and choices:
        message = choices[0].get("message") if isinstance(choices[0], dict) else None
        if isinstance(message, dict) and "content" in message:
            return message["content"]
    if "text" in response_json:
        return response_json["text"]
    raise RunnerError(
        "EXECUTION_FAILED", "Could not find response text in API response."
    )


def call_api(request_body, api_key, timeout):
    data = json.dumps(request_body).encode("utf-8")
    req = urllib.request.Request(
        DEEPROUTER_ROUTING_URL,
        data=data,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        raise RunnerError("EXECUTION_FAILED", f"DeepRouter API returned HTTP {e.code}.")
    except (urllib.error.URLError, TimeoutError, OSError) as e:
        raise RunnerError("EXECUTION_FAILED", f"Failed to reach DeepRouter API: {e}")
    try:
        return json.loads(body)
    except json.JSONDecodeError:
        raise RunnerError("EXECUTION_FAILED", "DeepRouter API returned invalid JSON.")


def run(user_input):
    root = package_root()
    manifest = read_manifest(root)
    validate_manifest(manifest)
    read_skill_md(root)
    api_key = get_api_key()
    timeout = get_timeout()
    request_body = build_request_body(user_input, manifest)
    response_json = call_api(request_body, api_key, timeout)
    return parse_response(response_json)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    args = parser.parse_args()

    try:
        output = run(args.input)
    except RunnerError as e:
        print(format_error(e.code, e.message, e.cta), file=sys.stderr)
        sys.exit(1)

    print(output)
    sys.exit(0)


if __name__ == "__main__":
    main()
