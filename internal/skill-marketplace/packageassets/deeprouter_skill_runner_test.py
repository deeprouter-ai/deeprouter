"""Unit tests for the pure functions in deeprouter_skill_runner.py.

Covers request-body assembly and response-parsing fallback logic — the one
part of the runner that the Go subprocess black-box tests (runner_test.go)
cannot reach, because the actual API call goes to a hardcoded production URL
(see packaging.go) and can't be pointed at a mock server in CI.

Stdlib only, matching the runner's own zero-dependency requirement — this
file is never bundled into a shipped package (see embed.go), so that
constraint isn't strictly load-bearing here, but there's no reason to
introduce a third-party test framework for four small functions.

Run: python3 -m unittest deeprouter_skill_runner_test.py -v
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import deeprouter_skill_runner as runner  # noqa: E402


class TestBuildRequestBody(unittest.TestCase):
    def test_includes_user_input_and_manifest_ids(self):
        manifest = {"skill_id": 1, "skill_version_id": 42}
        body = runner.build_request_body("hello", manifest)
        self.assertEqual(body["messages"], [{"role": "user", "content": "hello"}])
        self.assertEqual(body["deeprouter"], {"skill_id": 1, "skill_version_id": 42})

    def test_ignores_extra_manifest_fields(self):
        manifest = {
            "skill_id": 1,
            "skill_version_id": 42,
            "slug": "demo",
            "requires_deeprouter_key": True,
        }
        body = runner.build_request_body("hi", manifest)
        self.assertEqual(set(body.keys()), {"messages", "deeprouter"})


class TestParseResponse(unittest.TestCase):
    def test_prefers_openai_shape(self):
        resp = {"choices": [{"message": {"content": "answer"}}]}
        self.assertEqual(runner.parse_response(resp), "answer")

    def test_falls_back_to_top_level_text(self):
        resp = {"text": "fallback answer"}
        self.assertEqual(runner.parse_response(resp), "fallback answer")

    def test_prefers_choices_over_text_when_both_present(self):
        resp = {
            "choices": [{"message": {"content": "from choices"}}],
            "text": "from text",
        }
        self.assertEqual(runner.parse_response(resp), "from choices")

    def test_empty_choices_list_falls_back_to_text(self):
        resp = {"choices": [], "text": "fallback"}
        self.assertEqual(runner.parse_response(resp), "fallback")

    def test_choices_present_but_no_message_content_falls_back_to_text(self):
        resp = {"choices": [{"message": {}}], "text": "fallback"}
        self.assertEqual(runner.parse_response(resp), "fallback")

    def test_raises_execution_failed_when_neither_shape_present(self):
        with self.assertRaises(runner.RunnerError) as ctx:
            runner.parse_response({"unexpected": "shape"})
        self.assertEqual(ctx.exception.code, "EXECUTION_FAILED")


class TestFormatError(unittest.TestCase):
    def test_omits_cta_when_not_given(self):
        import json

        payload = json.loads(runner.format_error("EXECUTION_FAILED", "boom"))
        self.assertEqual(payload, {"code": "EXECUTION_FAILED", "message": "boom"})

    def test_includes_cta_when_given(self):
        import json

        payload = json.loads(
            runner.format_error("AUTH_REQUIRED", "need a key", cta="go register")
        )
        self.assertEqual(
            payload,
            {"code": "AUTH_REQUIRED", "message": "need a key", "cta": "go register"},
        )


if __name__ == "__main__":
    unittest.main()
