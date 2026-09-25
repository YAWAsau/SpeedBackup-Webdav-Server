"""Exercise the release CLI using a disposable root; prints no credentials."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile

exe = str(Path(sys.argv[1]).resolve())
with tempfile.TemporaryDirectory(prefix="sb-password-policy-") as temp:
    root = Path(temp) / "data"
    subprocess.run([exe, "init", "--root", str(root)], check=True, capture_output=True)
    args = [exe, "admin-reset", "--root", str(root), "--username", "admin", "--password-stdin"]
    for password in ["x", "密" * 800, " x "]:
        subprocess.run(args, input=(password + "\n").encode(), check=True, capture_output=True)
        versions = sorted((root / ".speedbackup-server" / "administrators").glob("*.json"))
        account = json.loads(versions[-1].read_text(encoding="utf-8"))
        expected = hashlib.pbkdf2_hmac("sha256", password.encode(), bytes.fromhex(account["salt"]), 210000).hex()
        assert account["password_hash"] == expected, "CLI changed or truncated the password"
    before = sorted((root / ".speedbackup-server" / "administrators").glob("*.json"))
    for invalid in [b"\n", b" \t\n", b"x" * 70000 + b"\n"]:
        assert subprocess.run(args, input=invalid, capture_output=True).returncode != 0
        assert sorted((root / ".speedbackup-server" / "administrators").glob("*.json")) == before
print(json.dumps({"short_unicode_long_and_spaced_passwords": "PASS", "blank_and_oversized_input_leave_account_unchanged": "PASS"}))
