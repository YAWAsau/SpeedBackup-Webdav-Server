"""Run only as root in a disposable VM with the test service/data."""
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.request

assert os.geteuid() == 0
package = str(Path(sys.argv[1]).resolve())
installer = str(Path(__file__).resolve().parents[1] / "install_linux.sh")
def run(*args, check=True):
    return subprocess.run(args, check=check, capture_output=True, text=True, timeout=120)
def state(command):
    return run("systemctl", command, "--quiet", "speedbackup-server", check=False).returncode == 0
def install(mode):
    run("sh", installer, "--autostart", mode, package)

results = {}
install("off")
assert not state("is-enabled") and state("is-active")
results["install_manual_starts_current_service"] = True
run("systemctl", "stop", "speedbackup-server")
install("keep")
assert not state("is-enabled") and not state("is-active")
results["upgrade_keep_preserves_disabled_and_stopped"] = True
install("on")
assert state("is-enabled") and state("is-active")
with urllib.request.urlopen("http://127.0.0.1:8765/api/v1/capabilities", timeout=5) as response:
    assert json.load(response)["server"] == "SpeedBackup Server"
results["install_auto_starts_and_enables"] = True
run("/usr/bin/speedbackup-server", "service", "autostart", "--enabled=false")
assert not state("is-enabled") and state("is-active")
run("/usr/bin/speedbackup-server", "service", "autostart", "--enabled=true")
assert state("is-enabled") and state("is-active")
results["CLI_changes_boot_policy_without_stopping"] = True
assert run("sh", installer, "--autostart", "invalid", package, check=False).returncode != 0
assert state("is-enabled") and state("is-active")
results["invalid_choice_has_no_effect"] = True
print(json.dumps(results, indent=2))
