"""Run as root only in a disposable Linux VM after test_linux_install.py."""
from pathlib import Path
import hashlib,http.cookiejar,json,os,pwd,subprocess,sys,time,urllib.request,urllib.error
assert os.geteuid()==0
package=sys.argv[1];results={};root=Path('/var/lib/speedbackup-server/data')
def run(*args,input=None,check=True):return subprocess.run(args,input=input,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=120,check=check)
def ready():
 for _ in range(100):
  try:
   with urllib.request.urlopen('http://127.0.0.1:8765/api/v1/auth/status',timeout=2) as r:return json.load(r)
  except OSError:time.sleep(.1)
 raise AssertionError('server not ready')
try:
 run('dpkg','-i',package);run('systemctl','enable','--now','speedbackup-server');ready()
 share=Path('/srv/webdav2-share');blocked=Path('/srv/webdav2-blocked')
 assert not share.exists() and not blocked.exists(), 'refusing existing fixture directories'
 share.mkdir(mode=0o700);blocked.mkdir(mode=0o700);user=pwd.getpwnam('speedbackup');os.chown(share,user.pw_uid,user.pw_gid)
 output=run('python3',str(Path(__file__).with_name('smoke_webdav_accounts.py')),'--url','http://127.0.0.1:8765','--share',str(share),'--blocked-share',str(blocked)).stdout
 results['service_account_webdav']=json.loads(output)
 payload=share/'測試備份/user.tar.zst';before=hashlib.sha256(payload.read_bytes()).hexdigest()
 args=['runuser','-u','speedbackup','--','/usr/bin/speedbackup-server','admin-reset','--root',str(root),'--username','testadmin','--password-stdin']
 assert run(*args,input='recovered-test-password\n',check=False).returncode!=0
 results['reset_refuses_running_server']=True
 run('systemctl','stop','speedbackup-server');run(*args,input='recovered-test-password\n');run('systemctl','start','speedbackup-server');ready()
 client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 def login(password):
  r=urllib.request.Request('http://127.0.0.1:8765/api/v1/auth/login',data=json.dumps({'username':'testadmin','password':password}).encode(),headers={'X-SB-Admin':'1','Content-Type':'application/json'})
  try:
   with client.open(r) as response:return response.status
  except urllib.error.HTTPError as e:return e.code
 assert login('isolated-development-password')==401
 assert login('recovered-test-password')==200
 results['password_recovery']=True
 run('dpkg','-i',package);ready();assert login('recovered-test-password')==200
 with client.open('http://127.0.0.1:8765/api/v1/admin/webdav') as r:account=json.load(r)['users'][0]
 assert account['directory']==str(share) and account['username']=='phone'
 assert hashlib.sha256(payload.read_bytes()).hexdigest()==before
 results['reinstall_preserves_admin_mapping_and_payload']=True
 run('dpkg','--remove','speedbackup-server')
 assert hashlib.sha256(payload.read_bytes()).hexdigest()==before
 assert list((root/'.speedbackup-server/administrators').glob('*.json'))
 results['uninstall_preserves_shared_data_and_accounts']=True
except Exception as e:
 results['error']=str(e);raise
finally:
 Path('/root/webdav2/linux-webdav-accounts.json').write_text(json.dumps(results,indent=2))
 print(json.dumps(results,indent=2))
