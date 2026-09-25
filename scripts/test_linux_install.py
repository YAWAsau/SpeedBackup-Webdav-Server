"""Run only as root on a disposable Debian/Ubuntu VM; installs/removes package."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import time
import urllib.request

p = argparse.ArgumentParser()
p.add_argument('--package', required=True)
p.add_argument('--old-package')
args = p.parse_args()
results = {}
base = Path('/var/lib/speedbackup-server')
report = Path('/root/linux-install-results.json')

def run(*command, check=True, env=None):
    result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=120, env=env)
    if check and result.returncode:
        raise RuntimeError(f'{command[0]} failed ({result.returncode}): {result.stdout}')
    return result

def ready():
    for _ in range(100):
        try:
            with urllib.request.urlopen('http://127.0.0.1:8765/api/v1/capabilities', timeout=2) as r:
                assert json.load(r)['server'] == 'SpeedBackup Server'
            return
        except OSError:
            time.sleep(.1)
    raise AssertionError('HTTP not ready')

try:
    assert os.geteuid() == 0
    assert not base.exists(), 'Refusing to run over an existing installation'
    first = args.old_package or args.package
    run('dpkg', '-i', first)
    ready()
    results['fresh_install'] = True
    token_path = base / 'FIRST_RUN_TOKEN.txt'
    token = token_path.read_text().strip()
    assert token.startswith('sb1_') and (token_path.stat().st_mode & 0o777) == 0o600
    assert token_path.stat().st_uid == 0
    env = dict(os.environ, SPEEDBACKUP_SERVER_TOKEN=token)
    smoke = run('sh', str(Path(__file__).with_name('smoke_linux.sh')), env=env)
    assert 'SMOKE OK' in smoke.stdout
    results['api_upload_commit_download'] = True
    run('dpkg', '-i', args.package)
    ready()
    assert token_path.read_text().strip() == token
    results['upgrade_preserves_token_and_data'] = True
    run('systemctl', 'stop', 'speedbackup-server')
    with socket.socket(socket.AF_INET6) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
        sock.bind(('::', 8765))
        sock.listen()
        failed = run('systemctl', 'start', 'speedbackup-server', check=False)
        assert failed.returncode != 0, 'Start reported success despite occupied port'
        run('systemctl', 'stop', 'speedbackup-server')
        failed = run('dpkg', '-i', args.package, check=False)
        assert failed.returncode != 0, 'Package install reported success despite startup failure'
        results['installer_failure_exit_code'] = True
        run('systemctl', 'stop', 'speedbackup-server')
    results['port_conflict_reports_failure'] = True
    run('dpkg', '--configure', 'speedbackup-server')
    run('systemctl', 'start', 'speedbackup-server')
    ready()
    config = Path('/etc/speedbackup-server/server.env')
    config.write_text(config.read_text() + '\n# operator-customization-preserved\n')
    config_hash = hashlib.sha256(config.read_bytes()).hexdigest()
    run('dpkg', '-i', args.package)
    assert hashlib.sha256(config.read_bytes()).hexdigest() == config_hash
    results['reinstall_preserves_custom_config'] = True
    run('systemctl', 'disable', '--now', 'speedbackup-server')
    run('dpkg', '-i', args.package)
    assert run('systemctl', 'is-enabled', '--quiet', 'speedbackup-server', check=False).returncode != 0
    assert run('systemctl', 'is-active', '--quiet', 'speedbackup-server', check=False).returncode != 0
    results['reinstall_preserves_disabled_service'] = True
    manifests = list((base/'data/.speedbackup-server/manifests').rglob('*.json'))
    assert manifests
    before = {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in manifests}
    run('dpkg', '--remove', 'speedbackup-server')
    assert run('systemctl', 'is-active', '--quiet', 'speedbackup-server', check=False).returncode != 0
    assert not Path('/usr/bin/speedbackup-server').exists()
    assert token_path.read_text().strip() == token
    assert all(hashlib.sha256(Path(path).read_bytes()).hexdigest() == digest for path, digest in before.items())
    results['remove_preserves_data'] = True
    run('dpkg', '--purge', 'speedbackup-server')
    assert token_path.exists() and all(Path(path).exists() for path in before)
    results['purge_preserves_data'] = True
except Exception as error:
    results['error'] = str(error)
    raise
finally:
    report.write_text(json.dumps(results, indent=2))
    print(json.dumps(results, indent=2))
