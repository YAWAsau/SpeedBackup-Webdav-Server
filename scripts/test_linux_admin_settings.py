"""Run only as root in a disposable systemd VM containing test data."""
import http.cookiejar
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import urllib.error
import urllib.request

assert os.geteuid() == 0
package = str(Path(sys.argv[1]).resolve())
unit = 'speedbackup-server.service'
root = '/var/lib/speedbackup-server/data'
result = {}
def run(*args, check=True, **kw):
    return subprocess.run(args, check=check, capture_output=True, text=True, timeout=120, **kw)
def prop(name):
    return run('systemctl','show','--value','-p',name,unit).stdout.strip()
run('dpkg','-i',package)
run('systemctl','stop',unit)
run('runuser','-u','speedbackup','--','/usr/bin/speedbackup-server','admin-reset','--root',root,'--username','uitest','--password-stdin',input='x\n')
run('systemctl','start',unit)
client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def req(path, data=None, expected=200):
    r=urllib.request.Request('http://127.0.0.1:8765'+path,data=json.dumps(data).encode() if data is not None else None,headers={'X-SB-Admin':'1','Content-Type':'application/json'})
    try: response=client.open(r,timeout=20)
    except urllib.error.HTTPError as e: response=e
    with response:
        body=json.load(response)
        assert response.status==expected,(response.status,body)
        return body
req('/api/v1/auth/login',{'username':'uitest','password':'x'})
initial=req('/api/v1/admin/autostart');assert initial['manageable'],initial
pid=prop('MainPID');assert int(pid)>0
for enabled in [False,True,False]:
    state=req('/api/v1/admin/autostart',{'enabled':enabled})
    assert state['enabled']==enabled and state['manageable'],state
    assert prop('UnitFileState')==('enabled' if enabled else 'disabled')
    assert prop('MainPID')==pid and prop('ActiveState')=='active'
result['API_toggle_real_systemd_nonroot_service_same_PID']=True
assert req('/api/v1/admin/directories/picker')['available'] is False
req('/api/v1/admin/directories/picker',{},409)
result['Linux_uses_server_directory_browser']=True
# Root may use the fixed local protocol, but unknown fields cannot target units.
before=prop('UnitFileState')
with socket.socket(socket.AF_UNIX) as sock:
    sock.settimeout(10);sock.connect('/run/speedbackup-server-autostart.sock')
    sock.sendall(b'{"enabled":true,"unit":"ssh.service"}\n')
    assert json.loads(sock.recv(4096))['error']
assert prop('UnitFileState')==before and prop('MainPID')==pid
result['helper_rejects_extra_commands']=True
# Same UID but unrelated PID must not impersonate the HTTP service.
code="import socket; s=socket.socket(socket.AF_UNIX); s.settimeout(10); s.connect('/run/speedbackup-server-autostart.sock'); s.sendall(b'{\"enabled\":true}\\n'); print(s.recv(4096))"
denied=run('runuser','-u','speedbackup','--','python3','-c',code,check=False)
assert prop('UnitFileState')==before and '"error": ""' not in denied.stdout
result['helper_rejects_unrelated_same_UID_process']=True
run('dpkg','-i',package)
assert prop('UnitFileState')=='disabled'
run('systemctl','start',unit)
assert prop('ActiveState')=='active'
req('/api/v1/auth/login',{'username':'uitest','password':'x'})
assert req('/api/v1/admin/autostart')['manageable']
result['upgrade_preserves_disabled_policy_and_helper_works']=True
marker=Path(root)/'webdav8-preserved.txt';marker.write_text('keep test backup')
run('dpkg','--remove','speedbackup-server')
assert not Path('/run/speedbackup-server-autostart.sock').exists()
assert marker.read_text()=='keep test backup'
run('dpkg','--purge','speedbackup-server')
assert not Path('/lib/systemd/system/speedbackup-server-autostart.socket').exists()
assert not Path('/lib/systemd/system/speedbackup-server-autostart@.service').exists()
assert marker.read_text()=='keep test backup'
result['remove_purge_cleans_helper_preserves_backups']=True
run('dpkg','-i',package)
run('systemctl','start',unit)
req('/api/v1/auth/login',{'username':'uitest','password':'x'})
state=req('/api/v1/admin/autostart');assert state['manageable'] and state['enabled'],state
assert marker.read_text()=='keep test backup'
result['fresh_package_after_purge_ready']=True
print(json.dumps(result,indent=2))
