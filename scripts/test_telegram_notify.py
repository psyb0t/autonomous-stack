"""Exercise progress payloads and failures without contacting Telegram."""

import json
import os
import unittest
from unittest.mock import MagicMock, patch
from urllib.error import URLError

from telegram_notify import send


class NotifyTests(unittest.TestCase):
    def setUp(self):
        environment = patch.dict(os.environ, {
            "CLAUDEBOX_TELEGRAM_MODE_TOKEN": "123:EXAMPLE_DO_NOT_USE",
            "TELEGRAM_CHAT_ID": "42",
        }, clear=True)
        environment.start()
        self.addCleanup(environment.stop)

    @patch("telegram_notify.urllib.request.urlopen")
    def test_payload(self, open_url):
        response = MagicMock()
        response.read.return_value = b'{"ok":true}'
        open_url.return_value.__enter__.return_value = response
        for text in ['Starting: <plain> $literal é', 'x' * 4096, '😀' * 2048]:
            send(text)
            payload = json.loads(open_url.call_args.args[0].data)
            self.assertEqual(payload["chat_id"], 42)
            self.assertEqual(payload["text"], text)
            self.assertNotIn("parse_mode", payload)
            self.assertTrue(payload["link_preview_options"]["is_disabled"])
            self.assertEqual(open_url.call_args.kwargs["timeout"], 15)

    @patch("telegram_notify.urllib.request.urlopen")
    def test_invalid_inputs_do_not_send(self, open_url):
        for text in ["", "   ", "x" * 4097, "😀" * 2049]:
            with self.subTest(length=len(text)), self.assertRaises(ValueError):
                send(text)
        for key, value in [("TELEGRAM_CHAT_ID", "0"),
                           ("TELEGRAM_CHAT_ID", "42/evil"),
                           ("CLAUDEBOX_TELEGRAM_MODE_TOKEN", "")]:
            with patch.dict(os.environ, {key: value}), self.assertRaises(ValueError):
                send("Starting work")
        open_url.assert_not_called()

    @patch("telegram_notify.urllib.request.urlopen")
    def test_provider_failure_does_not_expose_token(self, open_url):
        open_url.side_effect = URLError("https://api.telegram.org/botSECRET/sendMessage")
        with self.assertRaisesRegex(RuntimeError, "^Telegram progress delivery failed$"):
            send("Starting work")

    @patch("telegram_notify.urllib.request.urlopen")
    def test_rejected_and_malformed_responses(self, open_url):
        response = MagicMock()
        open_url.return_value.__enter__.return_value = response
        for body in [b'{"ok":false}', b'[]', b'invalid', b'x' * 65537]:
            response.read.return_value = body
            with self.subTest(length=len(body)), self.assertRaises(RuntimeError):
                send("Starting work")
