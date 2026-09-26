const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync(require('node:path').join(__dirname,'../internal/sbserver/web/app.js'),'utf8');
const start=source.indexOf('  let stopDavMonitor='),end=source.indexOf('  async function renderWebDAV()',start);
assert(start>0&&end>start);
const listeners=new Map(),calls=[];let aborted=0,painted=0,destroyed=0;
const updates=[];
const ctx=vm.createContext({
  S:{page:'dashboard',locale:'zh-TW'},$:()=>({}),SBDAVMonitor:{create:()=>({update:rows=>{updates.push(rows);painted++;return 0;},error:()=>{},destroy:()=>destroyed++})},formatBytes:String,formatDuration:String,
  document:{hidden:false,addEventListener:(n,f)=>listeners.set(n,f),removeEventListener:(n,f)=>{assert.equal(listeners.get(n),f);listeners.delete(n);}},
  performance,setTimeout,clearTimeout,AbortController,
  api:(path,{signal})=>{
    const call={path};calls.push(call);
    if(calls.length===1)return Promise.resolve({transfers:[{id:1,state:'transfer_complete'}],revision:'0'});
    return new Promise((resolve,reject)=>{
      call.resolve=resolve;
      signal.addEventListener('abort',()=>{aborted++;const e=new Error('aborted');e.name='AbortError';reject(e);},{once:true});
    });
  }
});
vm.runInContext(source.slice(start,end)+'\nglobalThis.testStart=startDAVMonitor;globalThis.testStop=()=>stopDavMonitor();',ctx);
const until=async(test)=>{const deadline=Date.now()+1000;while(!test()){assert(Date.now()<deadline,'monitor did not progress');await new Promise(r=>setTimeout(r,5));}};
(async()=>{
  await ctx.testStart('dashboard',0);await until(()=>calls.length===2);
  assert(!listeners.has('scroll'),'page scrolling must not pause statistics');
  assert.equal(calls[1].path,'/api/v1/admin/webdav/activity?watch=1&after=0&delta=1');
  await new Promise(r=>setTimeout(r,20));assert.equal(calls.length,2,'must wait for server event response');
  calls[1].resolve({transfers:[{id:2,state:'transferring'}],partial:true,revision:'0'});await until(()=>calls.length===3);
  assert.equal(updates.at(-1).length,2,'delta discarded completed history');assert.equal(updates.at(-1)[1].id,1);
  ctx.document.hidden=true;listeners.get('visibilitychange')();await until(()=>aborted===1);
  await new Promise(r=>setTimeout(r,20));assert.equal(calls.length,3,'hidden page started a request');
  ctx.document.hidden=false;listeners.get('visibilitychange')();await until(()=>calls.length===4);
  calls[3].resolve({transfers:[],revision:'2'});await until(()=>calls.length===5);
  assert.equal(updates.at(-1).length,0,'full resync retained stale history');
  assert.equal(calls[4].path,'/api/v1/admin/webdav/activity?watch=1&after=2&delta=1');assert(painted>0);
  ctx.testStop();await until(()=>aborted===2);assert.equal(listeners.size,0);assert.equal(destroyed,1);
  await new Promise(r=>setTimeout(r,20));assert.equal(calls.length,5,'stopped monitor restarted');
  console.log('PASS: event long-poll revision, no idle polling loop, hidden/visible cancellation, page teardown');
})().catch(e=>{console.error(e);process.exitCode=1;ctx.testStop();});
