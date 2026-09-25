const assert=require('node:assert/strict');
const request=require('../internal/sbserver/web/request.js');
const original=global.fetch;
function stall(signal){return new Promise((_,reject)=>{if(signal.aborted)reject(new DOMException('Aborted','AbortError'));else signal.addEventListener('abort',()=>reject(new DOMException('Aborted','AbortError')),{once:true});});}
(async()=>{
 let calls=0;
 global.fetch=async(_,opts)=>{calls++;assert.equal(opts.headers.get('X-SB-Admin'),'1');assert.equal(opts.body,'{"a":1}');return {ok:true,headers:new Headers({'content-type':'application/json'}),json:async()=>({saved:true})};};
 const options={method:'POST',body:{a:1}};assert.deepEqual(await request('/test',options),{saved:true});assert.deepEqual(options.body,{a:1});
 global.fetch=(_,opts)=>{calls++;return stall(opts.signal);};
 await assert.rejects(request('/test',{method:'POST',body:{a:1},timeoutMs:10}),{name:'TimeoutError'});assert.equal(calls,2,'writes must not retry automatically');
 global.fetch=async(_,opts)=>({ok:true,headers:new Headers({'content-type':'application/json'}),json:()=>stall(opts.signal)});
 await assert.rejects(request('/test',{timeoutMs:10}),{name:'TimeoutError'});
 const abort=new AbortController();const result=request('/test',{signal:abort.signal,timeoutMs:1000});abort.abort();await assert.rejects(result,{name:'AbortError'});
 await assert.rejects(request('/test',{signal:abort.signal}),{name:'AbortError'});
 global.fetch=async()=>({ok:false,status:409,headers:new Headers({'content-type':'application/json'}),json:async()=>({message:'conflict'})});
 await assert.rejects(request('/test'),{status:409,message:'conflict'});
 console.log('PASS: write no-retry, body deadline, external cancellation, status and immutable options');
})().catch(e=>{console.error(e);process.exitCode=1;}).finally(()=>{global.fetch=original;});
