(function(root){
  'use strict';
  // Include body decoding in the deadline: receiving headers is not completion.
  async function request(path, options={}) {
    const {timeoutMs=15000, signal, ...opts}=options;
    const controller=new AbortController();
    const abort=()=>controller.abort();
    let timedOut=false;
    if(signal?.aborted) abort();
    else signal?.addEventListener('abort',abort,{once:true});
    const timer=setTimeout(()=>{timedOut=true;abort();},timeoutMs);
    try {
      const headers=new Headers(opts.headers||{});headers.set('X-SB-Admin','1');
      if(opts.body && typeof opts.body!=='string' && !(opts.body instanceof Blob)) {
        headers.set('Content-Type','application/json');opts.body=JSON.stringify(opts.body);
      }
      const res=await fetch(path,{...opts,headers,signal:controller.signal});
      const data=(res.headers.get('content-type')||'').includes('json')?await res.json():await res.text();
      if(!res.ok){const error=new Error(data?.message||`${res.status} ${res.statusText}`);error.status=res.status;throw error;}
      return data;
    } catch(error) {
      if(timedOut && !signal?.aborted){const timeout=new Error('Request timed out');timeout.name='TimeoutError';throw timeout;}
      throw error;
    } finally {clearTimeout(timer);signal?.removeEventListener('abort',abort);}
  }
  root.SBRequest=request;
  if(typeof module==='object')module.exports=request;
})(globalThis);
