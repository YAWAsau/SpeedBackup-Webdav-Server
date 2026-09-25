"""Disposable localhost test only. Share directories must be prepared by caller."""
import argparse,base64,hashlib,http.cookiejar,json,urllib.request,urllib.error
from urllib.parse import quote,urlsplit
p=argparse.ArgumentParser();p.add_argument('--url',required=True);p.add_argument('--share',required=True);p.add_argument('--blocked-share');a=p.parse_args()
assert urlsplit(a.url).hostname in ('localhost','127.0.0.1','::1'), 'test only on localhost'
jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
def req(method,path,body=None,headers=None):
 h={'X-SB-Admin':'1'};h.update(headers or {})
 if isinstance(body,dict):body=json.dumps(body).encode();h['Content-Type']='application/json'
 r=urllib.request.Request(a.url+path,data=body,headers=h,method=method)
 try:response=client.open(r,timeout=20)
 except urllib.error.HTTPError as e:response=e
 with response:return response.status,response.read()
creds={'username':'testadmin','password':'isolated-development-password'}
status=json.loads(req('GET','/api/v1/auth/status')[1]);assert status['setup_required'], 'refusing an already initialized administrator'
assert req('GET','/api/v1/admin/directories')[0]==401
assert req('POST','/api/v1/auth/setup',creds)[0]==200
assert req('POST','/api/v1/auth/setup',creds)[0]==409
assert req('POST','/api/v1/auth/logout')[0]==200
assert req('POST','/api/v1/auth/login',dict(creds,password='wrong-password'))[0]==401
assert req('POST','/api/v1/auth/login',creds)[0]==200
backup={'username':'phone','password':'isolated-backup-password','directory':a.share}
assert req('POST','/api/v1/admin/webdav',backup,{'X-SB-Admin':''})[0]==403
code,body=req('POST','/api/v1/admin/webdav',backup);assert code==200,(code,body)
if a.blocked_share:
 code,body=req('POST','/api/v1/admin/webdav',dict(backup,directory=a.blocked_share));assert code==400,(code,body)
assert req('GET','/api/v1/admin/directories?path='+quote(a.share,safe=''))[0]==200
dav=urllib.request.build_opener();auth='Basic '+base64.b64encode(b'phone:isolated-backup-password').decode()
def davreq(method,path,body=None,headers=None):
 h={'Authorization':auth};h.update(headers or {})
 r=urllib.request.Request(a.url+quote(path,safe='/'),data=body,headers=h,method=method)
 try:response=dav.open(r,timeout=20)
 except urllib.error.HTTPError as e:response=e
 with response:return response.status,response.read()
name='/dav/測試備份';assert davreq('MKCOL',name)[0]==201
payload=bytes(range(256))*8192;name+='/user.tar.zst'
assert davreq('PUT',name+'.part.test',payload)[0]==201
assert davreq('MOVE',name+'.part.test',headers={'Destination':a.url+quote(name,safe='/')})[0]==201
assert davreq('GET',name)[1]==payload
assert davreq('GET',name,headers={'Range':'bytes=33-199'})[1]==payload[33:200]
users=json.loads(req('GET','/api/v1/admin/webdav')[1])['users'];assert users[0]['custom_directory'] and users[0]['directory']==a.share
assert req('POST','/api/v1/auth/logout')[0]==200
assert req('GET','/api/v1/admin/directories')[0]==401
print(json.dumps({'result':'PASS','checks':['first setup','duplicate setup rejected','password login','logout invalidates session','CSRF rejected','custom directory browse/save','permission failure rejected' if a.blocked_share else 'permission writable checked','Chinese PUT/MOVE/GET','Range'],'payload_sha256':hashlib.sha256(payload).hexdigest()}))
