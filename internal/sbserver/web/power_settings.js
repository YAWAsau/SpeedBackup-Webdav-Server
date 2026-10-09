// Serialize writes so rapid appearance clicks cannot leave an older theme on the server.
globalThis.SBPowerThemeSync=(()=>{let queue=Promise.resolve(),last='';return (api,preferences)=>{const next=queue.catch(()=>{}).then(async()=>{const theme=preferences.all(),key=JSON.stringify(theme);if(key===last)return;await api('/api/v1/admin/power',{method:'POST',body:{action:'theme',theme}});last=key;});queue=next;return next;};})();
globalThis.SBPowerSettings={mount({container,api,current,preferences}){
 const panel=document.createElement('div');panel.className='section pad dav-gap';
 panel.innerHTML='<h3>備份完成後關機</h3><p class="muted">整次作業收尾後由腳本請求；失敗或部分完成時，必須先由使用者明確確認。Windows 原生視窗顯示 60 秒倒數，可取消或延後；其他傳輸進行中會等待。</p><label class="check-label"><input type="checkbox" id="powerEnabled" disabled>允許已授權腳本請求關機（預設關閉）</label><p id="powerStatus" role="status">正在讀取…</p><label class="check-label"><input type="checkbox" id="pairingEnabled">允許裝置配對（首次需在電腦批准）</label><p class="muted tiny">支援匿名 WebDAV。手機與電腦比對完整指紋後授權；之後免重複配對。</p><h4>已配對手機與待批准裝置</h4><p class="muted tiny">刪除會立即作廢授權；手機下次請求時須重新在電腦批准。</p><div id="powerDevices"></div><button class="btn" data-power="revoke_pairs">撤銷目前短效憑證</button><div class="power-controls"><button class="btn" data-power="rotate">舊版：建立／更新關機授權</button> <button class="btn" data-power="preview">預覽提示（不會關機）</button></div><p class="muted tiny">授權只顯示一次，更新後舊授權立即失效並停用自動關機。新版 DEX 使用裝置配對，不需複製此舊版憑證。</p><input class="input hidden" id="powerToken" readonly aria-label="新的關機授權" autocomplete="off"><p id="powerJob" role="status"></p><div id="powerJobActions" class="hidden"><button class="btn" data-power="defer">延後 10 分鐘</button> <button class="btn" data-power="cancel">取消關機</button></div>';
 container.append(panel);let state=null,working=false;
 const q=id=>panel.querySelector('#'+id);
 function draw(v){if(!current())return;state=v;q('pairingEnabled').checked=!!v.pairing_enabled;q('pairingEnabled').disabled=!v.supported||!v.ready;q('powerDevices').replaceChildren();if(!Object.keys(v.devices||{}).length)q('powerDevices').textContent='目前沒有已配對手機或待批准裝置';for(const d of Object.values(v.devices||{})){const row=document.createElement('div');row.className='dav-gap';const name=document.createElement('p');name.textContent=(d.name||'未命名裝置')+' · '+(d.approved?'已授權':d.revoked?'已撤銷':'等待批准');const fp=document.createElement('code');fp.textContent=d.id;fp.style.overflowWrap='anywhere';row.append(name,fp);for(const [a,label] of (d.approved||d.revoked?[['delete_device','刪除授權']]:[['approve_device','核對指紋後允許'],['revoke_device','拒絕'],['delete_device','刪除']])){const b=document.createElement('button');b.className='btn';b.textContent=label;b.onclick=()=>action(a,d.id);row.append(b);}q('powerDevices').append(row);}q('powerEnabled').checked=v.enabled;q('powerEnabled').disabled=!v.supported||!v.configured||!v.ready;
  q('powerStatus').textContent=!v.supported?'目前僅支援 Windows 桌面提示':!v.ready?'關機設定無法讀取，功能已鎖定':v.enabled?'已允許備份後關機':'備份後關機已關閉';
  for(const b of panel.querySelectorAll('[data-power]'))b.disabled=!v.supported;
  const j=v.job,pending=j&&['waiting','countdown','deferred'].includes(j.state);
  q('powerJob').textContent=j?(j.preview?'預覽 · ':'')+j.message+(j.deadline_ms?' · '+Math.max(0,Math.ceil((j.deadline_ms-v.server_ms)/1000))+' 秒':''):'';
  q('powerJobActions').classList.toggle('hidden',!pending);
  if(v.token){q('powerToken').classList.remove('hidden');q('powerToken').value=v.token;}
 }
 async function load(){try{draw(await api('/api/v1/admin/power'));}catch(e){if(current())q('powerStatus').textContent=e.message;}}
 async function action(action,device_id){if(working)return;working=true;try{if(action==='preview')await SBPowerThemeSync(api,preferences);draw(await api('/api/v1/admin/power',{method:'POST',body:{action,...(device_id?{device_id}:{}),job_id:state?.job?.job_id||'',...(action==='theme'?{theme:preferences.all()}:{})}}));}catch(e){if(current())q('powerStatus').textContent=e.message;}finally{working=false;}}
 q('pairingEnabled').onchange=()=>action(q('pairingEnabled').checked?'enable_pairing':'disable_pairing');
 q('powerEnabled').onchange=()=>action(q('powerEnabled').checked?'enable':'disable');
 for(const b of panel.querySelectorAll('[data-power]'))b.onclick=()=>action(b.dataset.power);
 const timer=setInterval(()=>{if(!current()){clearInterval(timer);q('powerToken').value='';return;}if(!working)void load();},2000);void load();
}};
