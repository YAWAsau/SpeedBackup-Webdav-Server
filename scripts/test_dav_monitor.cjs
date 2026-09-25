const assert=require('node:assert/strict');
require('../internal/sbserver/web/dav_monitor.js');
const {presentation,windowRange}=globalThis.SBDAVMonitor;
const say=tw=>tw,bytes=n=>n+' B',duration=n=>Math.floor(n)+'s';
const stream={state:'transferring',operation:'PUT',bytes:512,expected:-1,started_ms:1000,ended_ms:0,username:'phone',file:'user.tar.zst'};
let p=presentation(stream,1250,say,bytes,duration);
assert.equal(p.pct,null);assert.match(p.fields[5],/總大小尚未確定/);assert.equal(p.fields[7],'0.25s');
p=presentation({...stream,state:'transfer_complete',ended_ms:1500},2000,say,bytes,duration);
assert.equal(p.pct,100);assert.equal(p.fields[4],'512 B');assert.match(p.fields[5],/實際大小/);
p=presentation({...stream,state:'failed',ended_ms:1500},2000,say,bytes,duration);
assert.equal(p.pct,null);assert.match(p.state,/失敗/);assert.match(p.fields[5],/已傳輸大小/);
p=presentation({...stream,expected:1024},1500,say,bytes,duration);assert.equal(p.pct,50);
for(const count of [0,1,50,200,10000])for(let top=0;top<count*88;top+=413){const r=windowRange(count,top,480);assert(r.end-r.start<=13);assert(r.start>=0&&r.end<=count);}
console.log('PASS: truthful streamed/finished/failed sizes, subsecond duration, bounded visible row window');

for(const height of [88,232])for(let top=0;top<200*height;top+=173){const r=windowRange(200,top,480,height);assert(r.end-r.start<=(height===232?10:13));assert(r.start>=0&&r.end<=200);}
console.log("PASS: desktop/mobile virtual row sizing");

const {purpose,occurrence}=globalThis.SBDAVMonitor;
for(const [operation,file,destination,want] of [
 ['PUT','Backup/App/user.tar.zst.part.1789810489254.13','','備份數據上傳'],
 ['GET','Backup/App/apk.tar.zst','','恢復數據下載'],
 ['GET','Backup/apps.JSON','','獲取應用資訊'],
 ['GET','Backup/app_details_bundle.tar.zst','','獲取應用資訊'],
 ['PUT','Backup/apps.json.part.1789810489254.13','','更新 JSON 列表'],
 ['DELETE','Backup/apps.json','','刪除 JSON 列表'],
 ['DELETE','Backup/user.tar.gz','','刪除備份資料'],
 ['DELETE','Backup/App','','刪除檔案／目錄'],
 ['MOVE','Backup/user.tar.zst.part.123.4','Backup/user.tar.zst','提交備份數據'],
 ['MOVE','Backup/apps.json.part.123.4','Backup/apps.json','提交 JSON 列表'],
 ['MOVE','Backup/user.tar.zst.part.123.4','Other/user.tar.zst','移動／重新命名'],
 ['MOVE','Backup/folder','Backup/renamed','移動／重新命名'],
 ['COPY','Backup/folder','Backup/copy','複製檔案／目錄'],
 ['MKCOL','Backup/App','','建立目錄'],
 ['GET','Backup/data.bin','','檔案下載'],
 ['PUT','Backup/data.bin','','檔案上傳'],
 ['PUT','Backup/user.tar.zst.part.notes','','檔案上傳'],
])assert.equal(purpose({operation,path:file,destination},say),want);
const started=Date.UTC(2026,8,20,12,34,56,123);
process.env.TZ='UTC';assert.deepEqual(occurrence(started),{date:'2026-09-20',time:'12:34:56.123 +00:00',full:'2026-09-20 12:34:56.123 +00:00'});
process.env.TZ='Asia/Kolkata';assert.equal(occurrence(started).time,'18:04:56.123 +05:30');
for(const value of [0,NaN,undefined,-1])assert.equal(occurrence(value).full,'—');
p=presentation({...stream,operation:'MOVE',path:'old.json',destination:'new.json',state:'operation_complete',started_ms:started,ended_ms:started+25,http_status:201},started+30,say,bytes,duration);
assert.equal(p.pct,null);assert.equal(p.fields[4],'—');assert.equal(p.fields[6],'—');assert.equal(p.fields[3],'old.json → new.json');assert.equal(p.state,'移動完成');assert.match(p.timeTitle,/56\.148/);
p=presentation({...stream,operation:'DELETE',state:'failed',http_status:404},1500,say,bytes,duration);
assert.equal(p.state,'刪除失敗');assert.equal(p.fields[8],'HTTP 404');
console.log('PASS: operation purpose, staged metadata/archives, MOVE destinations, precise local timestamps, honest mutation states');
