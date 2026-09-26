(function(root){
  'use strict';
  const choices={theme:['system','dark','black','white'],accent:['blue','teal','violet'],density:['comfortable','compact'],motion:['system','reduce'],surface:['classic','glass']};
  const defaults={theme:'system',accent:'blue',density:'comfortable',motion:'system',surface:'classic'};
  const read=(key)=>{try{return localStorage.getItem(key);}catch(_){return null;}};
  let current={...defaults,theme:read('sb_theme')||'system'};
  try{Object.assign(current,JSON.parse(read('sb_appearance')||'{}'));}catch(_){}
  function update(values){
    for(const key of Object.keys(defaults)){
      const value=values[key]??current[key];current[key]=choices[key].includes(value)?value:defaults[key];
      document.documentElement.dataset[key]=current[key];
    }
    let persisted=true;
    try{localStorage.setItem('sb_appearance',JSON.stringify(current));localStorage.setItem('sb_theme',current.theme);}catch(_){persisted=false;}
    return persisted;
  }
  root.SBPreferences={get:()=>current.theme,set:value=>{update({theme:value});return current.theme;},all:()=>({...current}),update,reset:()=>update(defaults),read};
  update(current);
})(globalThis);
