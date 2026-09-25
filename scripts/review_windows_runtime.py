"""Isolated Windows CLI/API regression checks; never installs a service."""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--exe", required=True)
    parser.add_argument("--expect-fixed", action="store_true")
    parser.add_argument("--onboarding-test-exe")
    args = parser.parse_args()
    exe = str(Path(args.exe).resolve())
    flags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
    results = {}

    def run(*command, **kwargs):
        return subprocess.run(command, capture_output=True, text=True,
                              encoding="utf-8", errors="replace", timeout=60,
                              creationflags=flags, **kwargs)

    with tempfile.TemporaryDirectory(prefix="speedbackup-server-review-") as td:
        base = Path(td)
        root = base / "collision-root"
        token_path = base / "existing-token.txt"
        token_path.write_text("operator-copy\n")
        first = run(exe, "init", "--root", str(root), "--token-file", str(token_path))
        assert first.returncode != 0 and "token file already exists" in first.stderr
        assert token_path.read_text() == "operator-copy\n"
        retry = run(exe, "init", "--root", str(root), "--token-file", str(base / "retry-token.txt"))
        results["token_file_failure_retry_created"] = retry.returncode == 0 and (base / "retry-token.txt").is_file()

        with socket.socket() as busy:
            busy.bind(("127.0.0.1", 0))
            busy.listen()
            occupied_root = base / "occupied-root"
            start = run(exe, "serve", "--root", str(occupied_root), "--listen", f"127.0.0.1:{busy.getsockname()[1]}")
            assert start.returncode != 0
            config_files = list((occupied_root / ".speedbackup-server" / "config").glob("*.json"))
            results["listen_failure_preserves_fresh_root"] = not config_files

        api_root = base / "api-root"
        api_token = base / "api-token.txt"
        init = run(exe, "init", "--root", str(api_root), "--token-file", str(api_token))
        assert init.returncode == 0, "API fixture initialization failed"
        token = api_token.read_text().strip()
        with socket.socket() as free:
            free.bind(("127.0.0.1", 0))
            port = free.getsockname()[1]
        url = f"http://127.0.0.1:{port}"
        process = subprocess.Popen([exe, "serve", "--root", str(api_root), "--listen", f"127.0.0.1:{port}"],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, creationflags=flags)
        try:
            for _ in range(100):
                try:
                    with urllib.request.urlopen(url + "/api/v1/capabilities", timeout=1) as response:
                        assert response.status == 200
                    break
                except (OSError, urllib.error.URLError):
                    assert process.poll() is None, "Server exited before readiness"
                    time.sleep(0.1)
            else:
                raise AssertionError("Server did not become ready")
            try:
                urllib.request.urlopen(url + "/api/v1/status", timeout=3)
                raise AssertionError("Unauthenticated status unexpectedly allowed")
            except urllib.error.HTTPError as err:
                assert err.code == 401
            results["unauthenticated_status_denied"] = True
            with urllib.request.urlopen(url + "/web/admin", timeout=3) as response:
                admin_html = response.read().decode("utf-8")
            assert ">SB<" not in admin_html
            assert admin_html.count('src="/web/admin/server-mark.svg"') == 2
            assert 'rel="icon"' in admin_html
            with urllib.request.urlopen(url + "/web/admin/server-mark.svg", timeout=3) as response:
                assert "image/svg+xml" in response.headers.get("Content-Type", "")
                assert b"<svg" in response.read()
            results["letter_free_brand_and_favicon"] = True
            if args.onboarding_test_exe:
                fixture = base / "onboarding"
                fixture.mkdir()
                fixture_token = fixture / "FIRST_RUN_TOKEN.txt"
                fixture_token.write_text(token, encoding="ascii")
                onboarding = run(str(Path(args.onboarding_test_exe).resolve()),
                                 "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART",
                                 f"/FIXTURE={fixture}", f"/PORT={port}")
                assert onboarding.returncode == 0, "Onboarding fixture failed"
                assert (fixture / "onboarding-test.txt").read_text().startswith("PASS:")
                assert fixture_token.read_text() == token
                results["installer_valid_stale_missing_token_controls"] = True
            env = dict(os.environ, BASE_URL=url, SPEEDBACKUP_SERVER_TOKEN=token)
            smoke = run("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File",
                        str(Path(__file__).with_name("smoke_windows.ps1").resolve()), env=env)
            assert smoke.returncode == 0, (smoke.stdout + smoke.stderr).replace(token, "[REDACTED]")
            results["nine_step_api_smoke"] = "SMOKE OK" in smoke.stdout
            second = run(exe, "init", "--root", str(api_root))
            results["running_root_lock_enforced"] = second.returncode != 0
        finally:
            process.terminate()
            process.communicate(timeout=10)
        # Forced exit above is test cleanup, not a graceful-service-stop test.
    print(json.dumps(results, indent=2))
    if args.expect_fixed:
        assert all(results.values()), "A runtime regression check failed"


if __name__ == "__main__":
    main()
