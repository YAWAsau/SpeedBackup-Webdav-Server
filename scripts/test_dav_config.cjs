const assert = require('node:assert/strict');
require('../internal/sbserver/web/dav_config.js');
const {config, initialAddress} = globalThis.SBWebDAV;
const address='http://192.168.0.8:8765/';
const simple=config(address,'phone','x');
assert.equal(simple, `remote_type=webdav\nwebdav_url='${address}'\nwebdav_remote_user='phone'\nwebdav_remote_pass='x'`);
assert.equal(config(address,'phone',`a'$HOME\"\\$(echo fail) 中文`).split('\n')[3], `webdav_remote_pass='a'"'"'$HOME"\\$(echo fail) 中文'`);
assert.throws(()=>config(address,'phone','\n'), /line/);
assert.throws(()=>config(address,'phone',''), /password/);
assert.throws(()=>config('javascript:alert(1)','phone','x'), /url/);
assert.throws(()=>config('http://user:secret@example.com/dav/','phone','x'), /url/);
assert.throws(()=>config(address,'Bad User','x'), /user/);
for(const origin of ['http://127.0.0.1:8765','http://localhost:18765','http://[::1]:8765']) assert.equal(initialAddress(origin,'/dav/',{preferred_url:address}),address);
assert.equal(initialAddress('https://backup.example.com','/dav/',{preferred_url:address}),'https://backup.example.com/dav/');
assert.equal(initialAddress('http://10.0.0.4:8765','/dav/',{preferred_url:address}),'http://10.0.0.4:8765/dav/');
assert.equal(initialAddress('http://localhost:8765','/dav/',{}),'http://localhost:8765/dav/');
assert.equal(initialAddress('http://localhost:8765','/dav/',{selection_required:true}), '');
assert.throws(()=>config('http://127.0.0.1:8765/dav/','phone','x'), /loopback/);
console.log('PASS: script keys, quoting, validation, IPv4 defaults and external URL preservation');

assert.equal(SBWebDAV.shareAddress(address,'guest',true),'http://192.168.0.8:8765/dav-public/guest/');
assert.equal(SBWebDAV.shareAddress('https://example.com/prefix/dav-public/old/','guest',true),'https://example.com/prefix/dav-public/guest/');
assert.equal(SBWebDAV.shareAddress('http://192.168.0.8:8765/dav-public/guest/','guest',false),address);
assert.equal(config('http://192.168.0.8:8765/dav-public/guest/','guest','',true).split('\n').slice(-2).join('\n'),"webdav_remote_user=''\nwebdav_remote_pass=''");
console.log('PASS: anonymous URL and empty script credentials');

for (const root of ['http://192.168.0.205:8765', 'http://192.168.0.205:8765/']) {
  assert.equal(SBWebDAV.shareAddress(root,'root',true),'http://192.168.0.205:8765/dav-public/root/');
  assert.equal(config(root,'root','',true).split('\n')[1],"webdav_url='http://192.168.0.205:8765/dav-public/root/'");
  assert.equal(config(root,'root','x').split('\n')[1],"webdav_url='http://192.168.0.205:8765/'");
  assert.equal(config(root,'root','',true,'root').split('\n')[1],"webdav_url='http://192.168.0.205:8765/'");
}
assert.equal(config('https://backup.example.com','root','',true).split('\n')[1],"webdav_url='https://backup.example.com/dav-public/root/'");
console.log('PASS: bare server address expands to the correct share, including copy before blur');
assert.equal(SBWebDAV.shareAddress('https://backup.example.com/dav-public/guest/','guest',true,'guest'),'https://backup.example.com/');
assert.equal(SBWebDAV.shareAddress('https://backup.example.com/prefix/dav/','phone',false),'https://backup.example.com/prefix/');
assert.equal(SBWebDAV.shareAddress('https://backup.example.com/','guest',true,'another'),'https://backup.example.com/dav-public/guest/');
console.log('PASS: standard root URL, unique anonymous selection, multiple share isolation and legacy URL conversion');
