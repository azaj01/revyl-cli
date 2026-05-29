from __future__ import annotations

import hashlib
import io
import os
import tempfile
import unittest
import urllib.error
from pathlib import Path
from unittest import mock

import revyl._binary as binary


class _FakeResponse(io.BytesIO):
    def __enter__(self) -> "_FakeResponse":
        return self

    def __exit__(self, exc_type, exc, tb) -> bool:
        self.close()
        return False


def _build_urlopen(binary_payload: bytes, checksums_payload: str):
    def _fake_urlopen(url, *args, **kwargs):
        target = getattr(url, "full_url", str(url))
        if target.endswith("/checksums.txt"):
            return _FakeResponse(checksums_payload.encode("utf-8"))
        if target.endswith("/revyl-linux-amd64"):
            return _FakeResponse(binary_payload)
        raise AssertionError(f"unexpected URL requested: {target}")

    return _fake_urlopen


class BinaryIntegrityTests(unittest.TestCase):
    def test_download_binary_verifies_checksum_and_writes_sidecar(self) -> None:
        payload = b"verified-binary"
        digest = hashlib.sha256(payload).hexdigest()
        checksums = f"{digest}  revyl-linux-amd64\n"

        with tempfile.TemporaryDirectory() as tmpdir:
            binary_path = Path(tmpdir) / "revyl-linux-amd64"
            with (
                mock.patch.object(binary, "get_platform_info", return_value=("linux", "amd64", "")),
                mock.patch.object(binary, "get_binary_path", return_value=binary_path),
                mock.patch("urllib.request.urlopen", side_effect=_build_urlopen(payload, checksums)),
            ):
                path = binary.download_binary()

            self.assertEqual(path, binary_path)
            self.assertTrue(binary_path.exists())
            self.assertEqual(binary_path.read_bytes(), payload)
            self.assertEqual((Path(str(binary_path) + ".sha256")).read_text(encoding="utf-8").strip(), digest)

    def test_download_binary_skips_verification_when_checksum_missing(self) -> None:
        payload = b"payload"
        checksums = "deadbeef  revyl-other-binary\n"

        with tempfile.TemporaryDirectory() as tmpdir:
            binary_path = Path(tmpdir) / "revyl-linux-amd64"
            with (
                mock.patch.object(binary, "get_platform_info", return_value=("linux", "amd64", "")),
                mock.patch.object(binary, "get_binary_path", return_value=binary_path),
                mock.patch("urllib.request.urlopen", side_effect=_build_urlopen(payload, checksums)),
            ):
                path = binary.download_binary()

            self.assertEqual(path, binary_path)
            self.assertTrue(binary_path.exists())
            self.assertEqual(binary_path.read_bytes(), payload)
            self.assertFalse(Path(str(binary_path) + ".sha256").exists())

    def test_download_binary_fails_when_checksum_mismatch(self) -> None:
        payload = b"payload"
        checksums = "0" * 64 + "  revyl-linux-amd64\n"

        with tempfile.TemporaryDirectory() as tmpdir:
            binary_path = Path(tmpdir) / "revyl-linux-amd64"
            with (
                mock.patch.object(binary, "get_platform_info", return_value=("linux", "amd64", "")),
                mock.patch.object(binary, "get_binary_path", return_value=binary_path),
                mock.patch("urllib.request.urlopen", side_effect=_build_urlopen(payload, checksums)),
            ):
                with self.assertRaises(RuntimeError):
                    binary.download_binary()

            self.assertFalse(binary_path.exists())

    def test_ensure_binary_redownloads_when_sidecar_missing(self) -> None:
        old_payload = b"old"
        new_payload = b"new-verified"
        new_digest = hashlib.sha256(new_payload).hexdigest()
        checksums = f"{new_digest}  revyl-linux-amd64\n"

        with tempfile.TemporaryDirectory() as tmpdir:
            binary_path = Path(tmpdir) / "revyl-linux-amd64"
            binary_path.write_bytes(old_payload)

            with (
                mock.patch.object(binary, "get_platform_info", return_value=("linux", "amd64", "")),
                mock.patch.object(binary, "get_binary_path", return_value=binary_path),
                mock.patch.dict(os.environ, {"PATH": ""}),
                mock.patch("urllib.request.urlopen", side_effect=_build_urlopen(new_payload, checksums)),
            ):
                resolved = binary.ensure_binary()

            self.assertEqual(resolved, binary_path)
            self.assertEqual(binary_path.read_bytes(), new_payload)
            self.assertEqual((Path(str(binary_path) + ".sha256")).read_text(encoding="utf-8").strip(), new_digest)

    def test_ensure_binary_uses_verified_existing_binary(self) -> None:
        payload = b"already-verified"
        digest = hashlib.sha256(payload).hexdigest()

        with tempfile.TemporaryDirectory() as tmpdir:
            binary_path = Path(tmpdir) / "revyl-linux-amd64"
            binary_path.write_bytes(payload)
            Path(str(binary_path) + ".sha256").write_text(digest + "\n", encoding="utf-8")

            with (
                mock.patch.object(binary, "get_binary_path", return_value=binary_path),
                mock.patch("urllib.request.urlopen") as mocked_urlopen,
            ):
                resolved = binary.ensure_binary()

            self.assertEqual(resolved, binary_path)
            mocked_urlopen.assert_not_called()

    def test_ensure_binary_finds_binary_on_system_path(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            sdk_path = Path(tmpdir) / "sdk" / "revyl-linux-amd64"
            system_path = Path(tmpdir) / "bin" / "revyl"
            sdk_path.parent.mkdir(parents=True, exist_ok=True)
            system_path.parent.mkdir(parents=True, exist_ok=True)
            system_path.write_bytes(b"system")
            system_path.chmod(0o755)

            with (
                mock.patch.object(binary, "get_binary_path", return_value=sdk_path),
                mock.patch.dict(os.environ, {"PATH": str(system_path.parent)}),
                mock.patch("urllib.request.urlopen") as mocked_urlopen,
            ):
                resolved = binary.ensure_binary()

            self.assertEqual(resolved, system_path.resolve())
            mocked_urlopen.assert_not_called()

    def test_ensure_binary_prefers_sdk_managed_over_system_path(self) -> None:
        payload = b"sdk-managed"
        digest = hashlib.sha256(payload).hexdigest()

        with tempfile.TemporaryDirectory() as tmpdir:
            sdk_path = Path(tmpdir) / "revyl-linux-amd64"
            sdk_path.write_bytes(payload)
            Path(str(sdk_path) + ".sha256").write_text(digest + "\n", encoding="utf-8")

            with (
                mock.patch.object(binary, "get_binary_path", return_value=sdk_path),
                mock.patch("shutil.which", return_value="/opt/homebrew/bin/revyl") as which_mock,
            ):
                resolved = binary.ensure_binary()

            self.assertEqual(resolved, sdk_path)
            which_mock.assert_not_called()

    def test_find_native_binary_skips_windows_pip_scripts_wrapper(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            python_dir = root / "Python312"
            scripts_dir = python_dir / "Scripts"
            native_dir = root / "native"
            scripts_dir.mkdir(parents=True)
            native_dir.mkdir()

            python_exe = python_dir / "python.exe"
            wrapper = scripts_dir / "revyl.exe"
            native = native_dir / "revyl.exe"
            python_exe.write_bytes(b"")
            wrapper.write_bytes(b"MZ wrapper")
            native.write_bytes(b"MZ native")
            wrapper.chmod(0o755)
            native.chmod(0o755)

            with (
                mock.patch.object(binary.sys, "platform", "win32"),
                mock.patch.object(binary.sys, "executable", str(python_exe)),
                mock.patch("sysconfig.get_path", return_value=str(scripts_dir)),
                mock.patch.dict(os.environ, {"PATH": os.pathsep.join([str(scripts_dir), str(native_dir)])}),
            ):
                resolved = binary._find_native_binary()

            self.assertEqual(resolved, native.resolve())

    def test_fetch_expected_checksum_returns_none_on_http_error(self) -> None:
        def _raise_404(url, *args, **kwargs):
            raise urllib.error.HTTPError(url, 404, "Not Found", {}, None)

        with mock.patch("urllib.request.urlopen", side_effect=_raise_404):
            result = binary._fetch_expected_checksum("revyl-linux-amd64")

        self.assertIsNone(result)

    def test_fetch_expected_checksum_returns_none_when_asset_not_in_checksums(self) -> None:
        checksums_payload = "deadbeef  revyl-other-binary\n"

        with mock.patch(
            "urllib.request.urlopen",
            return_value=_FakeResponse(checksums_payload.encode("utf-8")),
        ):
            result = binary._fetch_expected_checksum("revyl-linux-amd64")

        self.assertIsNone(result)

    def test_download_binary_succeeds_without_checksums(self) -> None:
        payload = b"unverified-binary"

        def _fake_urlopen(url, *args, **kwargs):
            target = getattr(url, "full_url", str(url))
            if target.endswith("/checksums.txt"):
                raise urllib.error.HTTPError(url, 404, "Not Found", {}, None)
            if target.endswith("/revyl-linux-amd64"):
                return _FakeResponse(payload)
            raise AssertionError(f"unexpected URL: {target}")

        with tempfile.TemporaryDirectory() as tmpdir:
            binary_path = Path(tmpdir) / "revyl-linux-amd64"
            with (
                mock.patch.object(binary, "get_platform_info", return_value=("linux", "amd64", "")),
                mock.patch.object(binary, "get_binary_path", return_value=binary_path),
                mock.patch("urllib.request.urlopen", side_effect=_fake_urlopen),
            ):
                path = binary.download_binary()

            self.assertEqual(path, binary_path)
            self.assertTrue(binary_path.exists())
            self.assertEqual(binary_path.read_bytes(), payload)
            self.assertFalse(Path(str(binary_path) + ".sha256").exists())


if __name__ == "__main__":
    unittest.main()
