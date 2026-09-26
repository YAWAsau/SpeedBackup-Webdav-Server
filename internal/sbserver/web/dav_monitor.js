(function(root){
  'use strict';
  const ROW_HEIGHT=88,MOBILE_ROW_HEIGHT=232,OVERSCAN=3;
  function purpose(x,say){
    // The script publishes through file.part.<milliseconds>.<counter>. Strip
    // only that trailing staging suffix, never a directory or middle substring.
    const original=String(x.file||x.path?.split('/').pop()||'').toLowerCase();
    const name=original.replace(/\.part(?:\.\d+(?:\.\d+)?)?$/,'');
    const info=/\.json(?:\.(?:gz|zst|zstd))?$/.test(name)||/^app_details(?:_bundle)?\.tar(?:\.(?:zst|zstd|gz|xz|bz2|lz4|br))?$/.test(name);
    const backup=/\.(?:tar(?:\.(?:zst|zstd|gz|xz|bz2|lz4|br))?|apk|zip|7z)$/.test(name);
    if(x.operation==='MOVE'){
      const source=String(x.path||x.file||''),target=String(x.destination||'');
      const staged=source.replace(/\.part(?:\.\d+(?:\.\d+)?)?$/i,'');
      if(staged!==source&&target===staged)return info?say('提交 JSON 列表','提交 JSON 列表'):backup?say('提交備份數據','提交备份数据'):say('提交上傳檔案','提交上传文件');
      return say('移動／重新命名','移动／重命名');
    }
    if(x.operation==='COPY')return say('複製檔案／目錄','复制文件／目录');
    if(x.operation==='MKCOL')return say('建立目錄','创建目录');
    if(x.operation==='DELETE')return info?say('刪除 JSON 列表','删除 JSON 列表'):backup?say('刪除備份資料','删除备份数据'):say('刪除檔案／目錄','删除文件／目录');
    if(x.operation==='GET')return info?say('獲取應用資訊','获取应用信息'):backup?say('恢復數據下載','恢复数据下载'):say('檔案下載','文件下载');
    if(x.operation==='PUT')return info?say('更新 JSON 列表','更新 JSON 列表'):backup?say('備份數據上傳','备份数据上传'):say('檔案上傳','文件上传');
    return String(x.operation||'—');
  }
  function occurrence(ms){
    const date=new Date(ms),pad=(n,w=2)=>String(n).padStart(w,'0');
    if(!Number.isFinite(ms)||ms<=0||Number.isNaN(date.getTime()))return {date:'—',time:'—',full:'—'};
    const offset=-date.getTimezoneOffset(),zone=(offset<0?'-':'+')+pad(Math.floor(Math.abs(offset)/60))+':'+pad(Math.abs(offset)%60);
    const day=date.getFullYear()+'-'+pad(date.getMonth()+1)+'-'+pad(date.getDate());
    const clock=pad(date.getHours())+':'+pad(date.getMinutes())+':'+pad(date.getSeconds())+'.'+pad(date.getMilliseconds(),3)+' '+zone;
    return {date:day,time:clock,full:day+' '+clock};
  }
  function windowRange(count,top,height,rowHeight=ROW_HEIGHT){
    const start=Math.max(0,Math.floor(top/rowHeight)-OVERSCAN);
    return {start,end:Math.min(count,Math.ceil((top+height)/rowHeight)+OVERSCAN)};
  }
  function presentation(x,now,say,formatBytes,formatDuration){
    const operation=!['GET','PUT'].includes(x.operation),done=x.state==='transfer_complete'||x.state==='operation_complete',failed=x.state==='failed';
    const sec=Math.max(.001,((x.ended_ms||now)-x.started_ms)/1000);
    const pct=operation?null:done?100:x.expected>0?Math.min(99.9,x.bytes/x.expected*100):null;
    const detail=done?say('已完成 · 實際大小','已完成 · 实际大小'):x.expected>=0?(pct===null?'0%':pct.toFixed(1)+'%'):
      failed?say('已傳輸大小','已传输大小'):say('串流傳輸 · 總大小尚未確定','流式传输 · 总大小尚未确定');
    const stamp=occurrence(x.started_ms),end=occurrence(x.ended_ms);
    const verb=x.operation==='DELETE'?say('刪除','删除'):x.operation==='MOVE'?say('移動','移动'):x.operation==='COPY'?say('複製','复制'):say('建立','创建');
    return {pct,fields:[purpose(x,say),x.username,x.app||x.backup||'—',x.destination?(x.path||x.file)+' → '+x.destination:x.file,
      operation?'—':formatBytes(x.bytes)+(!done&&x.expected>=0?' / '+formatBytes(x.expected):''),operation?(x.http_status?'HTTP '+x.http_status:say('處理中','处理中')):detail,operation?'—':formatBytes(x.bytes/sec)+'/s',sec<1?sec.toFixed(2)+'s':formatDuration(sec),x.error||(failed&&x.http_status?'HTTP '+x.http_status:''),stamp.date,stamp.time],
      timeTitle:say('伺服器開始：','服务器开始：')+stamp.full+(x.ended_ms?'\n'+say('結束：','结束：')+end.full:''),
      state:operation?verb+(done?say('完成','完成'):failed?say('失敗','失败'):say('中','中')):done?say('傳輸完成','传输完成'):failed?say('失敗 / 中斷','失败 / 中断'):say('傳輸中','传输中'),className:'badge '+(failed?'bad':'good')};
  }
  function create({stats,transfers,say,formatBytes,formatDuration}){
    const text=(node,value)=>{value=String(value);if(node.textContent!==value)node.textContent=value;};
    const statValues=[say('進行中','进行中'),say('本次運行上傳','本次运行上传'),say('本次運行下載','本次运行下载'),say('本次運行異常','本次运行异常')].map(label=>{
      const card=document.createElement('div');card.className='metric';
      const title=document.createElement('div');title.className='stat-label';title.textContent=label;
      const value=document.createElement('div');value.className='stat-value';value.textContent='—';
      card.append(title,value);stats.append(card);return value;
    });
    const wrap=document.createElement('div');wrap.className='dav-transfer-wrap';wrap.setAttribute('role','table');wrap.setAttribute('aria-label',say('WebDAV 傳輸','WebDAV 传输'));
    const head=document.createElement('div');head.className='dav-transfer-grid dav-transfer-head';head.setAttribute('role','row');
    for(const label of [say('用途 / 帳號','用途 / 账号'),say('應用 / 檔案','应用 / 文件'),say('傳輸進度','传输进度'),say('平均速度 / 耗時','平均速度 / 耗时'),say('狀態','状态'),say('發生時間（本地時區）','发生时间（本地时区）')]){
      const cell=document.createElement('div');cell.setAttribute('role','columnheader');cell.textContent=label;head.append(cell);
    }
    const viewport=document.createElement('div');viewport.className='dav-transfer-scroll';viewport.setAttribute('role','rowgroup');viewport.tabIndex=0;viewport.setAttribute('aria-label',say('傳輸清單，可捲動查看紀錄','传输列表，可滚动查看记录'));
    const space=document.createElement('div');space.className='dav-transfer-space';
    const body=document.createElement('div');body.className='dav-transfer-window';space.append(body);viewport.append(space);wrap.append(head,viewport);
    const empty=document.createElement('p');empty.className='empty';empty.textContent=say('尚無 WebDAV 操作；備份、恢復或刪除後會顯示。','尚无 WebDAV 操作；备份、恢复或删除后会显示。');
    const note=document.createElement('p');note.className='tiny muted';
    const error=document.createElement('p');error.className='alert error';error.hidden=true;error.setAttribute('role','status');
    transfers.append(wrap,empty,note,error);wrap.hidden=true;
    let order=[],pending=[],data=new Map(),now=0,frame=0,settleTimer,scrolling=false,destroyed=false;
    let rowHeight=typeof matchMedia==='function'&&matchMedia('(max-width:760px)').matches?MOBILE_ROW_HEIGHT:ROW_HEIGHT;
    const pool=[];
    function makeRow(){
      const node=document.createElement('div');node.className='dav-transfer-grid dav-transfer-row';node.setAttribute('role','row');
      node.innerHTML='<div role="cell"><div></div><div class="tiny muted"></div></div><div role="cell"><div></div><div class="tiny mono muted"></div></div><div role="cell"><div></div><div class="tiny muted"></div><div class="dav-progress" role="progressbar" aria-valuemin="0" aria-valuemax="100"><span></span></div></div><div role="cell"><div></div><div class="tiny muted"></div></div><div role="cell"><span class="badge"></span><div class="tiny muted"></div></div><div role="cell" class="dav-time"><div class="tiny mono"></div><div class="tiny mono muted"></div></div>';
      body.append(node);return {node,fields:node.querySelectorAll('[role="cell"]>div:not(.dav-progress)'),progress:node.querySelector('.dav-progress'),bar:node.querySelector('.dav-progress span'),badge:node.querySelector('.badge')};
    }
    function reorder(){
      if(order.length===pending.length&&order.every((id,i)=>id===pending[i]))return;
      // Keep the historical row under the reader's eyes when new files arrive.
      const top=viewport.scrollTop,index=Math.floor(top/rowHeight),anchor=order[index];
      order=pending;
      space.style.height=(order.length*rowHeight)+'px';viewport.style.height=Math.min(480,order.length*rowHeight)+'px';
      if(top>0&&anchor!==undefined){const next=order.indexOf(anchor);if(next>=0)viewport.scrollTop=next*rowHeight+top%rowHeight;}
      wrap.setAttribute('aria-rowcount',String(order.length+1));
    }
    function paint(){
      frame=0;if(destroyed)return;
      const range=windowRange(order.length,viewport.scrollTop,viewport.clientHeight||480,rowHeight),needed=range.end-range.start;
      while(pool.length<needed)pool.push(makeRow());
      body.style.transform=`translateY(${range.start*rowHeight}px)`;
      for(let i=0;i<pool.length;i++){
        const row=pool[i];row.node.hidden=i>=needed;if(i>=needed)continue;
        const id=order[range.start+i],x=data.get(id);if(!x){row.node.hidden=true;continue;}
        const previousID=row.node.dataset.transferId;
        row.node.dataset.transferId=id;row.node.setAttribute('aria-rowindex',String(range.start+i+2));
        const completedKey=x.ended_ms?JSON.stringify(x):null;
        if(completedKey&&row.completedKey===completedKey)continue;
        row.completedKey=completedKey;
        const value=presentation(x,now,say,formatBytes,formatDuration);
        value.fields.forEach((v,j)=>{text(row.fields[j],v);if(row.fields[j].title!==String(v))row.fields[j].title=String(v);});
        row.fields[9].title=value.timeTitle;row.fields[10].title=value.timeTitle;
        row.progress.hidden=value.pct===null;
        if(value.pct!==null){row.bar.style.transition=previousID===id&&!x.ended_ms?'transform 60ms linear':'none';row.bar.style.transform=`scaleX(${value.pct/100})`;row.progress.setAttribute('aria-valuenow',value.pct.toFixed(1));}
        row.progress.setAttribute('aria-label',x.file);text(row.badge,value.state);row.badge.className=value.className;
      }
    }
    function schedule(){if(!frame&&!destroyed)frame=requestAnimationFrame(paint);}
    function onScroll(){scrolling=true;clearTimeout(settleTimer);schedule();settleTimer=setTimeout(()=>{scrolling=false;reorder();schedule();},160);}
    viewport.addEventListener('scroll',onScroll,{passive:true});
    const resize=typeof ResizeObserver==='function'?new ResizeObserver(()=>{
      const next=typeof matchMedia==='function'&&matchMedia('(max-width:760px)').matches?MOBILE_ROW_HEIGHT:ROW_HEIGHT;
      if(next!==rowHeight){const index=viewport.scrollTop/rowHeight;rowHeight=next;space.style.height=(order.length*rowHeight)+'px';viewport.style.height=Math.min(480,order.length*rowHeight)+'px';viewport.scrollTop=index*rowHeight;}schedule();
    }):null;resize?.observe(viewport);
    function update(all,snapshotTime=Date.now(),summary){
      now=snapshotTime;const active=[],finished=[];let upload=0,download=0,failed=0;
      const nextData=new Map();
      for(const x of all){const id=String(x.id);nextData.set(id,x);(['transferring','processing'].includes(x.state)?active:finished).push(id);if(x.operation==='PUT')upload+=x.bytes;else if(x.operation==='GET')download+=x.bytes;if(x.state==='failed')failed++;}
      // During a scroll gesture, retain rows that just left the server's history.
      if(scrolling)for(const id of order)if(!nextData.has(id)&&data.has(id))nextData.set(id,data.get(id));
      data=nextData;pending=active.concat(finished);
      [summary?.active??active.length,formatBytes(summary?.upload_bytes??upload),formatBytes(summary?.download_bytes??download),summary?.failed??failed].forEach((v,i)=>text(statValues[i],v));
      wrap.hidden=!all.length;empty.hidden=!!all.length;
      if(!scrolling)reorder();schedule();
      const count=summary?.completed??finished.length-failed;
      text(note,say(`即時更新 · ${active.length?'操作進行中':'等待下一個操作'} · 本次運行已完成 ${count} 筆。保留最近 ${finished.length} 筆及所有進行中操作；累計於服務重啟時歸零。`,`实时更新 · ${active.length?'操作进行中':'等待下一个操作'} · 本次运行已完成 ${count} 条。保留最近 ${finished.length} 条及所有进行中操作；累计在服务重启时归零。`));
      error.hidden=true;return active.length;
    }
    return Object.freeze({update,error(message){text(error,say('更新中斷，顯示上次資料：','更新中断，显示上次数据：')+message);error.hidden=false;},destroy(){destroyed=true;cancelAnimationFrame(frame);clearTimeout(settleTimer);resize?.disconnect();viewport.removeEventListener('scroll',onScroll);}});
  }
  root.SBDAVMonitor=Object.freeze({create,presentation,windowRange,purpose,occurrence});
})(globalThis);
