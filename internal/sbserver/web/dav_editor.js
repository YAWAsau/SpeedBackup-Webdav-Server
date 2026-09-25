(function(root){
  'use strict';
  const key=x=>JSON.stringify(x);
  // Only explicit field completion submits a snapshot. Keystrokes never queue
  // a partially typed directory. A single writer prevents reordered responses.
  function create({initial=null,save,notify=()=>{}}){
    let saved=initial,pending=null,running=false,error=null,waiters=[];
    const emit=()=>notify({saved,busy:running,error});
    async function drain(){
      if(running)return;running=true;error=null;emit();
      while(pending){
        const next=pending;pending=null;
        if(key(next)===key(saved))continue;
        try{await save(next,saved);saved=next;error=null;}catch(e){error=e;pending=null;break;}
        emit();
      }
      running=false;emit();waiters.splice(0).forEach(resolve=>resolve());
    }
    return {
      submit(value){pending={...value};void drain();},
      dirty:value=>!saved||key(value)!==key(saved),
      busy:()=>running,
      settled:()=>running?new Promise(resolve=>waiters.push(resolve)):Promise.resolve(),
      reset(value){if(running)throw new Error('save pending');saved=value;error=null;pending=null;emit();},
    };
  }
  root.SBDAVEditor={create};
  if(typeof module==='object')module.exports=root.SBDAVEditor;
})(globalThis);
