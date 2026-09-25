const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
require('../internal/sbserver/web/dav_monitor.js');
const source=fs.readFileSync(path.join(__dirname,'../internal/sbserver/web/app.js'),'utf8');
const start=source.indexOf('  async function renderEvents(){'),end=source.indexOf('  async function renderSettings()',start);
assert(start>0&&end>start);
const escaped=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const started=Date.UTC(2026,8,20,15,2,54,490);
const rows=[{unix:1,event:'service_started',message:'ready'},
 ...['GET','PUT','MOVE','COPY','DELETE','MKCOL'].map((operation,i)=>({unix:1789916574,event:'webdav_transfer',message:i===4?'failed':'operation_complete',details:{operation,path:'backup/apps.json.part.123.4',file:operation==='MKCOL'?'folder':operation==='MOVE'?'apps.json.part.123.4':'apps.json',destination:operation==='MOVE'?'backup/apps.json':undefined,started_ms:started+i,ended_ms:started+i+10,http_status:i===4?404:201}})),
 {unix:1,event:'webdav_transfer',message:'<script>unsafe()</script>',details:{operation:'GET',file:'<img onerror=unsafe()>',started_ms:started}},
 {unix:1,event:'webdav_transfer',message:'legacy',details:{operation:'GET',file:'user.tar.zst'}}];
function context(locale,events){
 const content={innerHTML:''};
 const ctx=vm.createContext({S:{locale},davGeneration:1,SBDAVMonitor:globalThis.SBDAVMonitor,
  api:async url=>{assert.equal(url,'/api/v1/admin/events?limit=300');return events},
  $:id=>{assert.equal(id,'content');return content},esc:escaped,empty:()=>'<p>No events</p>',formatTime:n=>'legacy-'+n});
 vm.runInContext(source.slice(start,end)+'\nglobalThis.render=renderEvents;',ctx);
 return {ctx,content};
}
(async()=>{
 for(const locale of ['zh-TW','zh-CN']){
  const {ctx,content}=context(locale,rows);await ctx.render();
  const html=content.innerHTML;
  assert.equal((html.match(/class="event-row"/g)||[]).length,rows.length);
  for(const label of (locale==='zh-TW'?['獲取應用資訊','更新 JSON 列表','提交 JSON 列表','複製檔案／目錄','刪除 JSON 列表','建立目錄','恢復數據下載']:['获取应用信息','更新 JSON 列表','提交 JSON 列表','复制文件／目录','删除 JSON 列表','创建目录','恢复数据下载']))assert(html.includes(label),label);
  assert(html.includes(globalThis.SBDAVMonitor.occurrence(started).full));
  assert(html.includes('legacy-1'));assert(html.includes('&quot;http_status&quot;:404'));
  assert(!html.includes('<script>')&&!html.includes('<img '));assert(html.includes('&lt;script&gt;'));
 }
 const empty=context('zh-TW',[]);await empty.ctx.render();assert(empty.content.innerHTML.includes('No events'));
 const stale=context('zh-TW',rows);stale.ctx.api=async()=>{stale.ctx.davGeneration++;return rows};await stale.ctx.render();assert.equal(stale.content.innerHTML,'');
 console.log('PASS: real event renderer with WebDAV/non-WebDAV history, both locales, precise/legacy times, escaping, empty list and stale navigation');
})().catch(e=>{console.error(e);process.exitCode=1});
