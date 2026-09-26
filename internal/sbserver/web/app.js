(() => {
  'use strict';
  const I18N = {
    'zh-TW': {
      admin_user:'管理員帳號', admin_password:'密碼', confirm_password:'確認密碼', login_hint: '使用管理員帳號密碼登入。忘記密碼可在伺服器本機重設，不必保存登入 Token。', token: '管理 / API Token', login: '登入',
      webdav: '分享設定', webdav_sub: 'WebDAV 帳號、目錄與連線設定', dashboard: '儀表板', profiles: '分享與備份', sessions: '傳輸工作階段', manifests: 'Manifest', storage: '儲存空間', events: '事件日誌', settings: '設定',
      connected: '伺服器已連線', logout: '登出', refresh: '重新整理',
      dashboard_sub: '伺服器、協議與儲存狀態總覽', profiles_sub: '分享目錄與近期傳輸中的備份', sessions_sub: '備份與恢復的檔案傳輸進度及近期紀錄', manifest_sub: '查看目前 generation 的完整 manifest', storage_sub: 'CAS object store 與安全孤兒清理', events_sub: '伺服器端 audit events', settings_sub: '外觀、開機自啟、管理員密碼與伺服器資訊',
      uptime: '運行時間', objects: '物件數', object_bytes: '物件容量', generations: 'Generation', session_count: 'Sessions', profile_count: 'Profiles', protocol: '協議', version: '版本', root: '備份根目錄', listen: '監聽位址', server_info: '伺服器資訊', capabilities: 'Capabilities',
      device: '裝置', profile: 'Profile', generation: 'Generation', entries: '項目數', logical_size: '邏輯容量', state: '狀態', created: '建立時間', updated: '更新時間', uploaded: '已上傳', committed: '已提交', open: '進行中',
      choose_profile: '選擇裝置 / Profile', load_manifest: '載入 Manifest', search: '搜尋路徑 / App ID', path: '路徑', size: '大小', sha256: 'SHA-256', kind: '類型', app_id: 'App ID', no_data: '目前沒有資料',
      cleanup_plan: '掃描孤兒物件', cleanup_execute: '刪除孤兒物件', cleanup_warning: '刪除前會重新掃描所有 generation；只有沒有任何 manifest 引用的 CAS blob 才會刪除。', orphan_count: '孤兒物件', orphan_bytes: '孤兒容量', cleanup_done: '清理完成', confirm_cleanup: '確定刪除目前所有未被任何 generation 引用的孤兒物件？',
      language: '介面語言', auth_token: 'Token', rotate_token: '輪替 Token', rotate_warning: '輪替後舊 Token 立即失效。新 Token 只顯示一次。', rotate_confirm: '確定輪替伺服器 Token？目前 Android client / 其他瀏覽器會立即需要更新。', new_token: '新的 Token（只顯示一次）', copy: '複製',
      time: '時間', event: '事件', message: '訊息', details: '明細', login_failed: '登入失敗，請確認 Token。', request_failed: '請求失敗', copied: '已複製', refresh_ok: '已重新整理',
      feature_true: '支援', feature_false: '不支援', upload_objects: '上傳物件', logical_total: '邏輯總量',
    },
    'zh-CN': {
      admin_user:'管理员账号', admin_password:'密码', confirm_password:'确认密码', login_hint: '使用管理员账号密码登录。忘记密码可在服务器本机重置，不必保存登录 Token。', token: '管理 / API Token', login: '登录',
      webdav: '共享设置', webdav_sub: 'WebDAV 账号、目录与连接设置', dashboard: '仪表盘', profiles: '共享与备份', sessions: '传输会话', manifests: 'Manifest', storage: '存储空间', events: '事件日志', settings: '设置',
      connected: '服务器已连接', logout: '退出', refresh: '刷新',
      dashboard_sub: '服务器、协议与存储状态总览', profiles_sub: '共享目录与近期传输中的备份', sessions_sub: '备份与恢复的文件传输进度及近期记录', manifest_sub: '查看当前 generation 的完整 manifest', storage_sub: 'CAS object store 与安全孤儿清理', events_sub: '服务器端 audit events', settings_sub: '外观、开机自启、管理员密码与服务器信息',
      uptime: '运行时间', objects: '对象数', object_bytes: '对象容量', generations: 'Generation', session_count: 'Sessions', profile_count: 'Profiles', protocol: '协议', version: '版本', root: '备份根目录', listen: '监听地址', server_info: '服务器信息', capabilities: 'Capabilities',
      device: '设备', profile: 'Profile', generation: 'Generation', entries: '项目数', logical_size: '逻辑容量', state: '状态', created: '创建时间', updated: '更新时间', uploaded: '已上传', committed: '已提交', open: '进行中',
      choose_profile: '选择设备 / Profile', load_manifest: '载入 Manifest', search: '搜索路径 / App ID', path: '路径', size: '大小', sha256: 'SHA-256', kind: '类型', app_id: 'App ID', no_data: '当前没有数据',
      cleanup_plan: '扫描孤儿对象', cleanup_execute: '删除孤儿对象', cleanup_warning: '删除前会重新扫描所有 generation；只有没有任何 manifest 引用的 CAS blob 才会删除。', orphan_count: '孤儿对象', orphan_bytes: '孤儿容量', cleanup_done: '清理完成', confirm_cleanup: '确定删除当前所有未被任何 generation 引用的孤儿对象？',
      language: '界面语言', auth_token: 'Token', rotate_token: '轮换 Token', rotate_warning: '轮换后旧 Token 立即失效。新 Token 只显示一次。', rotate_confirm: '确定轮换服务器 Token？当前 Android client / 其他浏览器会立即需要更新。', new_token: '新的 Token（只显示一次）', copy: '复制',
      time: '时间', event: '事件', message: '消息', details: '详情', login_failed: '登录失败，请确认 Token。', request_failed: '请求失败', copied: '已复制', refresh_ok: '已刷新',
      feature_true: '支持', feature_false: '不支持', upload_objects: '上传对象', logical_total: '逻辑总量',
    }
  };

  const S = { setupRequired:false, localSetupAllowed:false, locale: SBPreferences.read('sb_locale') || 'zh-TW', page: 'dashboard', status: null, caps: null, storage: null, profiles: [] };
  const $ = (id) => document.getElementById(id);
  const esc = (v) => String(v ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
  const t = (k) => I18N[S.locale]?.[k] ?? I18N['zh-TW'][k] ?? k;

  function applyI18n() {
    document.documentElement.lang = S.locale === 'zh-CN' ? 'zh-Hans' : 'zh-Hant';
    document.querySelectorAll('[data-i18n]').forEach(el => el.textContent = t(el.dataset.i18n));
    $('localeSelect').value = S.locale;
    renderHeader();
  }
  function setLocale(locale) { if(!canLeave())return;S.locale = I18N[locale] ? locale : 'zh-TW'; try{localStorage.setItem('sb_locale', S.locale);}catch(_){} applyI18n(); if (!$('appView').classList.contains('hidden')) renderPage(); else drawLogin(); }
  function toast(msg) { const e=$('toast'); e.textContent=msg; e.classList.remove('hidden'); clearTimeout(toast._t); toast._t=setTimeout(()=>e.classList.add('hidden'),2600); }
  function formatBytes(n) { n=Number(n||0); const u=['B','KiB','MiB','GiB','TiB']; let i=0; while(n>=1024&&i<u.length-1){n/=1024;i++;} return `${n.toFixed(i?2:0)} ${u[i]}`; }
  function formatTime(sec) { if(!sec) return '—'; return new Date(Number(sec)*1000).toLocaleString(S.locale); }
  function formatDuration(sec) { sec=Number(sec||0); const d=Math.floor(sec/86400); sec%=86400; const h=Math.floor(sec/3600); sec%=3600; const m=Math.floor(sec/60); const s=Math.floor(sec%60); return [d?`${d}d`:null,h?`${h}h`:null,m?`${m}m`:null,`${s}s`].filter(Boolean).join(' '); }

  async function api(path, opts={}) {
    const say=(tw,sc)=>S.locale==='zh-CN'?sc:tw;
    try{return await SBRequest(path,opts);}
    catch(error){
      if(error.name==='TimeoutError')error.message=(opts.method && opts.method!=='GET')?
        say('連線逾時，尚未確認是否套用；請重新讀取狀態後再操作。','连接超时，尚未确认是否应用；请重新读取状态后再操作。'):
        say('讀取逾時，請檢查連線後重試。','读取超时，请检查连接后重试。');
      throw error;
    }
  }

  function drawLogin(){
    const cn=S.locale==='zh-CN';
    $('loginTitle').textContent=S.setupRequired?(cn?'建立管理员':'建立管理員'):(cn?'管理员登录':'管理員登入');
    $('confirmPasswordField').classList.toggle('hidden',!S.setupRequired);
    $('adminPassword').autocomplete=S.setupRequired?'new-password':'current-password';
    $('loginBtn').textContent=S.setupRequired?(cn?'建立账号并登录':'建立帳號並登入'):t('login');
    $('loginBtn').disabled=S.setupRequired&&!S.localSetupAllowed;
    if(S.setupRequired){
      document.querySelector('[data-i18n="login_hint"]').textContent=S.localSetupAllowed?
        (cn?'首次使用：设置管理员账号与密码。备份账号会在登录后另行创建。':'首次使用：設定管理員帳號與密碼。備份帳號會在登入後另行建立。'):
        (cn?'请先在服务器打开 http://127.0.0.1:端口/web/admin 建立管理员。无桌面的 Linux 可使用 SSH 本地端口转发，或本机 admin-reset 命令。':'請先在伺服器開啟 http://127.0.0.1:連接埠/web/admin 建立管理員。無桌面的 Linux 可使用 SSH 本機連接埠轉送，或本機 admin-reset 命令。');
    }
  }
  async function login() {
    const button=$('loginBtn');button.disabled=true;
    try {
      if(S.setupRequired&&$('adminPassword').value!==$('adminConfirm').value)throw new Error(S.locale==='zh-CN'?'两次密码不一致':'兩次密碼不一致');
      await api('/api/v1/auth/'+(S.setupRequired?'setup':'login'),{method:'POST',body:{username:$('adminUser').value.trim(),password:$('adminPassword').value}});
      $('adminPassword').value='';$('adminConfirm').value='';
      $('loginError').classList.add('hidden');$('loginView').classList.add('hidden');$('appView').classList.remove('hidden');
      await refreshCore();await renderPage();
    } catch(e) { $('loginError').textContent=e.message;$('loginError').classList.remove('hidden'); }
    finally {button.disabled=S.setupRequired&&!S.localSetupAllowed;}
  }
  async function logout() { try{await api('/api/v1/auth/logout',{method:'POST'});}finally{location.reload();} }
  async function initializeLogin(){
    sessionStorage.removeItem('sb_token');
    try{
      const info=await api('/api/v1/auth/status');S.setupRequired=info.setup_required;S.localSetupAllowed=info.local_setup_allowed;drawLogin();
      if(info.authenticated){$('loginView').classList.add('hidden');$('appView').classList.remove('hidden');await refreshCore();await renderPage();}
    }catch(e){$('loginError').textContent=e.message;$('loginError').classList.remove('hidden');$('loginBtn').disabled=true;}
  }
  async function refreshCore() {
    [S.status,S.caps] = await Promise.all([api('/api/v1/status'), api('/api/v1/capabilities')]);
  }

  const pageMeta = {
    webdav: ['webdav','webdav_sub'], dashboard: ['dashboard','dashboard_sub'], profiles:['profiles','profiles_sub'], sessions:['sessions','sessions_sub'], events:['events','events_sub'], settings:['settings','settings_sub']
  };
  function renderHeader(){ const [a,b]=pageMeta[S.page]||pageMeta.dashboard; $('pageTitle').textContent=t(a); $('pageSubtitle').textContent=t(b); }
  let shareEditor=null;
  function canLeave(){if(shareEditor?.busy()){toast(S.locale==='zh-CN'?'正在应用设置，请稍候。':'正在套用設定，請稍候。');return false;}return true;}
  function setPage(p){ if(p==='sessions')p='dashboard';if(!canLeave())return;S.page=p; document.querySelectorAll('.nav-item').forEach(b=>b.classList.toggle('active',b.dataset.page===(p==='webdav'?'profiles':p))); renderHeader(); renderPage(); }

  function metricStat(label, value, sub='') { return `<div class="metric"><div class="stat-label">${esc(label)}</div><div class="stat-value">${esc(value)}</div><div class="stat-sub">${esc(sub)}</div></div>`; }
  function empty(){ return `<div class="empty">${esc(t('no_data'))}</div>`; }

  async function renderDashboard(){
    const generation=davGeneration,say=(tw,sc)=>S.locale==='zh-CN'?sc:tw;
    const [,dav]=await Promise.all([refreshCore(),api('/api/v1/admin/webdav')]);
    if(generation!==davGeneration)return;
    const users=dav.users||[];
    $('content').innerHTML='<div class="section pad dashboard-panel">'+davMonitorMarkup()+`<div class="grid two dashboard-details">
      <div class="section pad"><h3>${t('server_info')}</h3><div class="kv">
        <div>${t('uptime')}</div><div>${formatDuration(S.status.uptime_seconds)}</div>
        <div>${t('version')}</div><div>${esc(S.status.version)}</div>
        <div>${t('listen')}</div><div class="mono">${esc(S.status.listen)}</div>
        <div>${say('服務資料目錄','服务数据目录')}</div><div class="mono">${esc(S.status.root)}</div>
      </div></div><div class="section pad"><h3>${say('WebDAV 分享','WebDAV 共享')}</h3><div class="stat-value">${users.filter(x=>!x.disabled).length}</div><p class="muted">${say('已啟用的分享；檔案存放於各分享目錄。','已启用的共享；文件存放在各共享目录。')}</p><button id="dashboardShares" class="btn ghost">${say('查看分享與備份','查看共享与备份')}</button><p class="tiny muted">${say('上方顯示實際傳輸量，不代表磁碟已佔用容量。','上方显示实际传输量，不代表磁盘已占用容量。')}</p></div></div></div>`;
    $('dashboardShares').onclick=()=>setPage('profiles');
    await startDAVMonitor('dashboard',generation);
  }

  async function renderProfiles(){
    const generation=davGeneration,say=(tw,sc)=>S.locale==='zh-CN'?sc:tw;
    const [dav,activity]=await Promise.all([api('/api/v1/admin/webdav'),api('/api/v1/admin/webdav/activity')]);
    if(generation!==davGeneration)return;
    const shares=(dav.users||[]).map(user=>{
      const backups=[...new Set(activity.transfers.filter(x=>x.username===user.username&&x.backup).map(x=>x.backup))];
      return `<tr><td data-label="${say('分享名稱','共享名称')}">${esc(user.username)}</td><td data-label="${t('state')}">${user.disabled?say('已停用','已停用'):user.anonymous?say('匿名分享','匿名共享'):say('帳密分享','账号密码共享')}</td><td class="mono" data-label="${say('分享目錄','共享目录')}">${esc(user.directory||say('預設帳號目錄','默认账号目录'))}</td><td data-label="${say('近期備份','近期备份')}">${backups.map(esc).join('<br>')||say('尚無可辨識的備份目錄','暂无可识别的备份目录')}</td></tr>`;
    }).join('');
    $('content').innerHTML=`<div class="section pad"><h3>${say('WebDAV 分享與近期備份','WebDAV 共享与近期备份')}</h3><div class="table-wrap"><table class="share-table"><thead><tr><th>${say('分享名稱','共享名称')}</th><th>${t('state')}</th><th>${say('分享目錄','共享目录')}</th><th>${say('近期傳輸中的備份名稱','近期传输中的备份名称')}</th></tr></thead><tbody>${shares||`<tr><td colspan="4">${say('尚未建立分享，請按「管理分享」設定。','尚未创建共享，请按“管理共享”设置。')}</td></tr>`}</tbody></table></div><p class="muted tiny">${say('備份名稱取自本次運行的近期傳輸路徑，並非磁碟完整清單。','备份名称取自本次运行的近期传输路径，并非磁盘完整列表。')}</p><button id="configureShares" class="btn ghost">${say('管理分享','管理共享')}</button></div>`;
    $('configureShares').onclick=()=>setPage('webdav');
  }

  async function renderEvents(){
    const generation=davGeneration;
    const say=(tw,sc)=>S.locale==='zh-CN'?sc:tw;
    const rows=await api('/api/v1/admin/events?limit=300');
    if(generation!==davGeneration)return;
    const html=rows.map(e=>{
      const dav=e.event==='webdav_transfer'&&e.details?.operation;
      const timestamp=dav&&e.details.started_ms?SBDAVMonitor.occurrence(e.details.started_ms).full:formatTime(e.unix);
      const label=dav?SBDAVMonitor.purpose(e.details,say)+' · '+e.message:e.message;
      return `<div class="event-row"><div>${esc(timestamp)}</div><div class="mono">${esc(e.event)}</div><div><strong>${esc(label)}</strong><div class="mono muted tiny">${esc(JSON.stringify(e.details||{}))}</div></div></div>`;
    }).join('');
    $('content').innerHTML=`<div class="toolbar"><a class="btn ghost" href="/api/v1/admin/diagnostics/export" download>${say('下載伺服器除錯包','下载服务器调试包')}</a></div><div class="section">${html||empty()}</div>`;
  }

  async function renderSettings(){
    const generation=davGeneration;
    await refreshCore();if(generation!==davGeneration)return;const cn=S.locale==='zh-CN';const say=(tw,sc)=>cn?sc:tw;
    $('content').innerHTML=`<div class="grid two"><div class="settings-stack"><div class="section pad"><h3>${t('language')}</h3><select id="settingsLocale" class="select"><option value="zh-TW">繁體中文</option><option value="zh-CN">简体中文</option></select><div class="field-label" id="settingsThemeLabel">${say('主題','主题')}</div></div><div class="section pad"><h3>${say('開機自動啟動','开机自动启动')}</h3><p id="autostartStatus" role="status">${say('正在讀取系統狀態…','正在读取系统状态…')}</p><label><input type="checkbox" id="autostartEnabled" disabled> ${say('開機時啟動伺服器','开机时启动服务器')}</label><p class="muted">${say('只影響下次開機，目前服務與傳輸會繼續運作。','只影响下次开机，当前服务与传输会继续运行。')}</p></div><div class="section pad"><h3>${say('從手機管理','从手机管理')}</h3><div id="adminLANLinks"></div></div></div><div class="section pad"><h3>${say('修改管理員密碼','修改管理员密码')}</h3><label class="field-label" for="currentAdminPassword">${say('目前密碼','当前密码')}</label><input id="currentAdminPassword" type="password" class="input" autocomplete="current-password"><label class="field-label" for="newAdminPassword">${say('新密碼','新密码')}</label><input id="newAdminPassword" type="password" class="input" autocomplete="new-password"><label class="field-label" for="confirmAdminPassword">${t('confirm_password')}</label><input id="confirmAdminPassword" type="password" class="input" autocomplete="new-password"><button id="changeAdminPassword" class="btn primary dav-gap">${say('更新密碼並重新登入','更新密码并重新登录')}</button><p id="passwordResult" role="status"></p></div></div><div class="section pad dav-gap"><p>${say('忘記密碼：在伺服器停止服務後，使用 admin-reset --password-stdin 重設，再啟動服務。備份帳號及檔案不受影響。','忘记密码：在服务器停止服务后，使用 admin-reset --password-stdin 重置，再启动服务。备份账号及文件不受影响。')}</p><div class="kv"><div>${t('version')}</div><div>${esc(S.status.version)}</div><div>${t('root')}</div><div class="mono">${esc(S.status.root)}</div></div></div>`;
    $('settingsLocale').value=S.locale;$('settingsLocale').onchange=e=>setLocale(e.target.value);
    const themeLabel=$('settingsThemeLabel');
    const appearance=document.createElement('div');appearance.className='appearance-options';
    appearance.innerHTML=`<div class="theme-previews" role="group" aria-labelledby="settingsThemeLabel">${[['system','系統','系统'],['dark','深色','深色'],['black','深黑','深黑'],['white','白色','白色']].map(([value,tw,cn])=>`<button type="button" class="theme-choice" data-preview="${value}" aria-pressed="false"><span class="theme-sample" aria-hidden="true"><i></i><i></i></span>${say(tw,cn)}</button>`).join('')}</div>
    <label class="field-label" for="settingsSurface">${say('介面材質','界面材质')}</label><select class="select" id="settingsSurface" aria-describedby="surfaceHint"><option value="classic">${say('經典（無玻璃效果）','经典（无玻璃效果）')}</option><option value="glass">${say('液態玻璃','液态玻璃')}</option></select><p id="surfaceHint" class="muted tiny">${say('柔和透光與高光邊緣；搭配任一主題。選擇經典可關閉效果。','柔和透光与高光边缘；搭配任一主题。选择经典可关闭效果。')}</p>
    <label class="field-label" for="settingsAccent">${say('強調色','强调色')}</label><select class="select" id="settingsAccent"><option value="blue">${say('藍色','蓝色')}</option><option value="teal">${say('青綠','青绿')}</option><option value="violet">${say('紫色','紫色')}</option></select>
    <label class="field-label" for="settingsDensity">${say('排列密度','排列密度')}</label><select class="select" id="settingsDensity"><option value="comfortable">${say('舒適','舒适')}</option><option value="compact">${say('緊湊（桌面）','紧凑（桌面）')}</option></select>
    <label class="check-label"><input id="settingsMotion" type="checkbox">${say('減少動畫','减少动画')}</label>
    <p class="muted">${say('外觀立即套用，只儲存在目前瀏覽器。手機保留適合觸控的間距。','外观立即应用，仅保存在当前浏览器。手机保留适合触控的间距。')}</p><button class="btn" id="resetAppearance">${say('還原外觀預設','恢复外观默认')}</button><p id="appearanceStatus" role="status" class="muted"></p><p class="muted tiny">OPPO Sans 4.0 · Copyright 2024 Guangdong OPPO Mobile Telecommunications Corp., Ltd. · <a href="/web/admin/OPPO-Sans-LICENSE.txt" target="_blank" rel="noopener">${say('字型授權','字体授权')}</a></p>`;
    // Render short appearance choices in-page, avoiding native popup compositing.
    appearance.querySelectorAll('select').forEach(select=>{
      const key=select.id.replace('settings','').toLowerCase();
      const label=appearance.querySelector(`label[for="${select.id}"]`);
      const group=document.createElement('div');group.id=select.id;group.className='appearance-choices';group.setAttribute('role','group');
      if(label){label.id=select.id+'Label';label.removeAttribute('for');group.setAttribute('aria-labelledby',label.id);}
      if(select.hasAttribute('aria-describedby'))group.setAttribute('aria-describedby',select.getAttribute('aria-describedby'));
      for(const option of select.options){const button=document.createElement('button');button.type='button';button.className='btn';button.dataset.preference=key;button.dataset.value=option.value;button.textContent=option.textContent;button.setAttribute('aria-pressed','false');group.appendChild(button);}
      select.replaceWith(group);
    });
    themeLabel.after(appearance);
    const syncAppearance=()=>{const p=SBPreferences.all();appearance.querySelectorAll('[data-preference]').forEach(b=>b.setAttribute('aria-pressed',String(p[b.dataset.preference]===b.dataset.value)));$('settingsMotion').checked=p.motion==='reduce';appearance.querySelectorAll('[data-preview]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.preview===p.theme)));};
    const appearanceResult=ok=>{syncAppearance();$('appearanceStatus').textContent=ok?say('已套用並儲存','已应用并保存'):say('已套用；瀏覽器禁止儲存，關閉後可能不保留。','已应用；浏览器禁止保存，关闭后可能不保留。');};
    const changeAppearance=values=>appearanceResult(SBPreferences.update(values));
    appearance.querySelectorAll('[data-preview]').forEach(b=>b.onclick=()=>changeAppearance({theme:b.dataset.preview}));
    appearance.querySelectorAll('[data-preference]').forEach(b=>b.onclick=()=>changeAppearance({[b.dataset.preference]:b.dataset.value}));
    $('settingsMotion').onchange=e=>changeAppearance({motion:e.target.checked?'reduce':'system'});
    $('resetAppearance').onclick=()=>appearanceResult(SBPreferences.reset());syncAppearance();
    const current=()=>generation===davGeneration&&S.page==='settings';
    let bootState=null;
    function drawBoot(state,message=''){
      if(!current())return;bootState=state;
      const box=$('autostartEnabled');box.checked=state?.enabled===true;box.indeterminate=state?.enabled==null;box.disabled=!state?.manageable;
      const reasons={portable:say('目前是手動啟動的程式；請使用已安裝的服務管理開機自啟。','当前是手动启动的程序；请使用已安装的服务管理开机自启。'),not_installed:say('尚未安裝系統服務。','尚未安装系统服务。'),helper_unavailable:say('開機控制元件尚未就緒，請重新安裝最新版服務。','开机控制组件尚未就绪，请重新安装最新版服务。'),permission_denied:say('服務沒有變更開機設定的權限。','服务没有更改开机设置的权限。'),unsupported_boot_policy:say('此服務由系統特殊策略管理，請由系統管理員調整。','此服务由系统特殊策略管理，请由系统管理员调整。'),systemd_unavailable:say('此環境未使用 systemd。','此环境未使用 systemd。'),unsupported:say('此平台不支援服務自啟控制。','此平台不支持服务自启控制。')};
      const status=state?.enabled==null?say('無法取得開機狀態','无法取得开机状态'):state.enabled?say('目前開機自啟：已開啟','当前开机自启：已开启'):say('目前開機自啟：已關閉','当前开机自启：已关闭');
      $('autostartStatus').textContent=[status,reasons[state?.reason]||state?.reason||'',message].filter(Boolean).join(' · ');
    }
    const loadBoot=async()=>{try{const state=await api('/api/v1/admin/autostart');drawBoot(state);}catch(e){drawBoot(null,e.message);}};
    $('autostartEnabled').onchange=async()=>{
      const enabled=$('autostartEnabled').checked;$('autostartEnabled').disabled=true;$('autostartStatus').textContent=say('正在套用…','正在应用…');
      try{drawBoot(await api('/api/v1/admin/autostart',{method:'POST',body:{enabled}}));}catch(e){let actual=null;try{actual=await api('/api/v1/admin/autostart');}catch(_){}drawBoot(actual,say('未確認套用結果：','未确认应用结果：')+e.message);}
    };
    void loadBoot();
    $('content').insertAdjacentHTML('beforeend',`<div class="section pad"><h3>${say('日誌與除錯','日志与调试')}</h3><p>${say('下載 ZIP 後可提供給維護者排查，傳輸無須停止。包含版本、近期操作、詳細請求與網頁錯誤；不包含備份內容、密碼、Token 或 Cookie。','下载 ZIP 后可提供给维护者排查，无须停止传输。包含版本、近期操作、详细请求与网页错误；不包含备份内容、密码、Token 或 Cookie。')}</p><div id="debugLogInfo" class="mono tiny"></div><p class="muted tiny">${say('請求日誌與操作日誌各保留 4 份，每份約 8 MiB。包含檔案路徑、帳號名稱及用戶端 IP；更早紀錄會輪替。','请求日志与操作日志各保留 4 份，每份约 8 MiB。包含文件路径、账号名称及客户端 IP；更早记录会轮替。')}</p><a id="downloadDebug" class="btn primary" href="/api/v1/admin/diagnostics/export" download>${say('下載伺服器除錯包','下载服务器调试包')}</a></div>`);
    api('/api/v1/admin/diagnostics').then(info=>{
      if(!current())return;
      $('debugLogInfo').textContent=say('詳細日誌：','详细日志：')+info.log_directory+'\n'+say('操作日誌：','操作日志：')+info.audit_directory+'\n'+say('丟失紀錄：','丢失记录：')+info.logging.dropped_records+' · '+say('寫入錯誤：','写入错误：')+info.logging.write_errors+(info.logging.last_write_error?' · '+info.logging.last_write_error:'');
    }).catch(e=>{if(current())$('debugLogInfo').textContent=e.message;});
    api('/api/v1/admin/webdav/connection').then(connection=>{
      if(!current())return;
      const urls=[...new Set((connection.addresses||[]).map(a=>new URL('/web/admin',a.url).href))];
      $('adminLANLinks').innerHTML=urls.length?urls.map(url=>`<p><a class="mono" href="${esc(url)}">${esc(url)}</a> <button class="btn ghost admin-copy" data-url="${esc(url)}">${t('copy')}</button></p>`).join(''):`<p>${say('尚無可用區網網址；服務需要監聽區網介面。','暂无可用局域网地址；服务需要监听局域网接口。')}</p>`;
      $('adminLANLinks').insertAdjacentHTML('beforeend',`<p class="muted">${say('手機與電腦連接同一區網，開啟上方網址並使用管理員帳密登入。127.0.0.1 只供伺服器本機使用。','手机与电脑连接同一局域网，打开上方地址并使用管理员账号密码登录。127.0.0.1 仅供服务器本机使用。')}</p>`);
      document.querySelectorAll('.admin-copy').forEach(button=>button.onclick=()=>copyText(button.dataset.url));
    }).catch(e=>{if(current())$('adminLANLinks').textContent=e.message;});
    $('changeAdminPassword').onclick=async()=>{
      const button=$('changeAdminPassword');button.disabled=true;
      try{if($('newAdminPassword').value!==$('confirmAdminPassword').value)throw new Error(say('兩次密碼不一致','两次密码不一致'));
        await api('/api/v1/admin/password',{method:'POST',body:{current_password:$('currentAdminPassword').value,new_password:$('newAdminPassword').value}});location.reload();
      }catch(e){$('passwordResult').textContent=e.message;}finally{button.disabled=false;}
    };
  }

  async function copyText(value){
    try{if(navigator.clipboard&&window.isSecureContext){await navigator.clipboard.writeText(value);}else{const box=document.createElement('textarea');box.value=value;document.body.appendChild(box);box.select();const ok=document.execCommand('copy');box.remove();if(!ok)throw new Error('clipboard unavailable');}toast(t('copied'));}catch(e){toast(S.locale==='zh-CN'?'请手动复制地址。':'請手動複製地址。');}
  }
  async function browseDirectory(generation){
    const say=(tw,sc)=>S.locale==='zh-CN'?sc:tw;
    const button=$('davBrowse');button.disabled=true;
    const alive=()=>generation===davGeneration&&S.page==='webdav';
    try{
      const capability=await api('/api/v1/admin/directories/picker');if(!alive())return;
      if(!capability.available){await chooseDirectory();return;}
      const job=await api('/api/v1/admin/directories/picker',{method:'POST',body:{directory:$('davDirectory').value.trim()}});if(!alive())return;
      const dialog=document.createElement('dialog');dialog.className='directory-dialog';
      dialog.innerHTML=`<h3>${say('Windows 資料夾選擇','Windows 文件夹选择')}</h3><p class="muted">${say('請在 Windows 視窗中選擇資料夾。瀏覽器若詢問是否開啟 SpeedBackup，請允許開啟。','请在 Windows 窗口中选择文件夹。浏览器若询问是否打开 SpeedBackup，请允许打开。')}</p><p class="picker-message" role="status"></p><div class="toolbar"><button class="btn ghost picker-fallback">${say('改用網頁瀏覽','改用网页浏览')}</button><button class="btn ghost picker-cancel">${say('取消','取消')}</button></div>`;
      document.body.appendChild(dialog);dialog.showModal();let timer,closed=false,finished=false;
      dialog.onclose=()=>{closed=true;clearTimeout(timer);dialog.remove();if(!finished)void api('/api/v1/admin/directories/picker/'+job.id,{method:'DELETE'}).catch(()=>{});};
      dialog.querySelector('.picker-cancel').onclick=()=>dialog.close();
      dialog.querySelector('.picker-fallback').onclick=()=>{dialog.close();if(alive())void chooseDirectory();};
      const poll=async()=>{
        if(closed)return;if(!alive()){dialog.close();return;}
        try{
          const state=await api('/api/v1/admin/directories/picker/'+job.id);if(closed||!alive())return;
          if(state.state==='selected'){finished=true;$('davDirectory').value=state.path;$('davDirectory').dispatchEvent(new Event('change'));dialog.close();return;}
          if(state.state==='cancelled'){finished=true;dialog.close();return;}
          if(state.state==='error'){finished=true;dialog.querySelector('.picker-message').textContent=state.error;return;}
          timer=setTimeout(poll,750);
        }catch(e){if(!closed)dialog.querySelector('.picker-message').textContent=e.message;}
      };
      const launch=document.createElement('a');launch.href=job.launch_url;document.body.appendChild(launch);launch.click();launch.remove();void poll();
    }catch(e){if(alive()){$('davSaveStatus').textContent=say('原生選擇器無法開啟，已切換網頁瀏覽：','原生选择器无法打开，已切换网页浏览：')+e.message;await chooseDirectory();}}
    finally{if(alive())button.disabled=false;}
  }
  async function chooseDirectory(){
    const cn=S.locale==='zh-CN';const say=(tw,sc)=>cn?sc:tw;
    const dialog=document.createElement('dialog');dialog.className='directory-dialog';
    dialog.innerHTML=`<h3>${say('選擇伺服器分享目錄','选择服务器共享目录')}</h3><p class="muted">${say('這是伺服器上的目錄；選定後會自動套用。','这是服务器上的目录；选定后会自动应用。')}</p><div class="toolbar"><input id="directoryPath" class="input mono" aria-label="${say('伺服器目錄路徑','服务器目录路径')}"><button id="directoryGo" class="btn ghost">${say('開啟','打开')}</button></div><p id="directoryCurrent" class="mono directory-current"></p><div id="directoryRoots" class="toolbar"></div><button id="directoryUp" class="btn ghost">${say('上一層','上一级')}</button><div id="directoryList" class="directory-list"></div><p id="directoryError" role="status"></p><div class="toolbar"><button id="directorySelect" class="btn primary">${say('使用此目錄','使用此目录')}</button><button id="directoryCancel" class="btn ghost">${say('取消','取消')}</button></div>`;
    document.body.appendChild(dialog);dialog.showModal();let current='',parent='',controller,requestNumber=0;
    const load=async(path)=>{const number=++requestNumber;controller?.abort();const requestController=new AbortController();controller=requestController;const timeout=setTimeout(()=>requestController.abort(),15000);$('directorySelect').disabled=true;try{
      const data=await api('/api/v1/admin/directories?path='+encodeURIComponent(path),{signal:requestController.signal});if(!dialog.isConnected||number!==requestNumber)return;current=data.path;parent=data.parent;$('directoryCurrent').textContent=current;
      $('directoryPath').value=current;$('directorySelect').disabled=!current;$('directoryUp').disabled=!current||current===parent;
      $('directoryRoots').innerHTML=data.roots.map(x=>`<button class="btn ghost directory-root" data-path="${esc(x.path)}">${esc(x.name)}</button>`).join('');
      $('directoryList').innerHTML=data.directories.map(x=>`<button class="directory-item" data-path="${esc(x.path)}">▸ ${esc(x.name)}</button>`).join('')||`<p class="muted">${say('沒有可列出的子目錄','没有可列出的子目录')}</p>`;
      dialog.querySelectorAll('[data-path]').forEach(b=>b.onclick=()=>load(b.dataset.path));
      $('directoryError').textContent=data.truncated?say('此目錄項目較多，僅列出前2000筆；可直接輸入完整路徑。','此目录项目较多，仅列出前2000条；可直接输入完整路径。'):'';
    }catch(e){if(!dialog.isConnected||number!==requestNumber)return;current='';$('directorySelect').disabled=true;$('directoryError').textContent=e.name==='AbortError'?say('目錄讀取逾時，請檢查磁碟連線或改選其他路徑。','目录读取超时，请检查磁盘连接或选择其他路径。'):e.message;}finally{clearTimeout(timeout);}};
    $('directoryGo').onclick=()=>load($('directoryPath').value);$('directoryPath').onkeydown=e=>{if(e.key==='Enter')load(e.target.value);};
    $('directoryUp').onclick=()=>load(parent);$('directoryCancel').onclick=()=>dialog.close();
    $('directorySelect').onclick=()=>{$('davDirectory').value=current;$('davDirectory').dispatchEvent(new Event('change'));dialog.close();};dialog.onclose=()=>{controller?.abort();dialog.remove();};
    await load($('davDirectory').value.trim());
  }

  let stopDavMonitor=()=>{}, davGeneration=0;
  function davMonitorMarkup(){
    const say=(tw,sc)=>S.locale==='zh-CN'?sc:tw;
    return `<div class="section pad"><h3>${say('WebDAV 操作紀錄','WebDAV 操作记录')}</h3><p class="muted">${say('依檔名辨識備份、恢復及應用資訊，並記錄移動、複製、建立目錄與刪除。時間為伺服器收到操作的時間，以本地時區顯示；下載完成不代表手機解壓或安裝已完成。','按文件名识别备份、恢复及应用信息，并记录移动、复制、创建目录与删除。时间为服务器收到操作的时间，以本地时区显示；下载完成不代表手机解压或安装已完成。')}</p><div id="davStats" class="grid stats dav-gap"></div><div id="davTransfers" class="dav-gap"></div></div>`;
  }
  async function startDAVMonitor(page,generation){
    if(S.page!==page||generation!==davGeneration)return;
    const view=SBDAVMonitor.create({stats:$('davStats'),transfers:$('davTransfers'),say:(tw,sc)=>S.locale==='zh-CN'?sc:tw,formatBytes,formatDuration});
    let stopped=false,timer,request,polling=false,revision=null,history=[];
    const alive=()=>!stopped&&S.page===page&&generation===davGeneration;
    const poll=async()=>{
      clearTimeout(timer);
      if(!alive()||document.hidden||polling)return;
      polling=true;request=new AbortController();let retryDelay=0;const started=performance.now();
      try{
        const watch=revision===null?'':'?watch=1&after='+encodeURIComponent(revision)+'&delta=1';
        const data=await api('/api/v1/admin/webdav/activity'+watch,{signal:request.signal,timeoutMs:35000});
        revision=typeof data.revision==='string'?data.revision:null;
        if(!alive()||document.hidden)return;
        if(!data.partial)history=data.transfers.filter(x=>!['transferring','processing'].includes(x.state));
        const active=view.update(data.partial?data.transfers.concat(history):data.transfers,data.snapshot_ms,data.summary);
        if(revision===null)retryDelay=active?50:5000;
      }catch(e){retryDelay=5000;if(alive()&&e.name!=='AbortError')view.error(e.message);}
      finally{polling=false;if(alive()&&!document.hidden)timer=setTimeout(poll,Math.max(retryDelay,50-(performance.now()-started)));}
    };
    const onVisibility=()=>{if(document.hidden){clearTimeout(timer);request?.abort();}else{revision=null;poll();}};
    document.addEventListener('visibilitychange',onVisibility);
    stopDavMonitor=()=>{stopped=true;clearTimeout(timer);request?.abort();view.destroy();document.removeEventListener('visibilitychange',onVisibility);};
    await poll();
  }
  async function renderWebDAV(){
    const generation=davGeneration;
    const cn=S.locale==='zh-CN';
    const say=(tw,sc)=>cn?sc:tw;
      const info=await api('/api/v1/admin/webdav');
      let rootAnonymous=info.root_anonymous_user||'';
    if(S.page!=='webdav'||generation!==davGeneration)return;
    let connection={addresses:[],preferred_url:'',loopback_only:false};
    try{connection=await api('/api/v1/admin/webdav/connection');}catch{connection.unavailable=true;}
    if(S.page!=='webdav'||generation!==davGeneration)return;
    const base=SBWebDAV.initialAddress(location.origin,info.path,connection);
    $('content').innerHTML=`
      <div class="section pad"><h3>${say('分享帳號與目錄','共享账号与目录')}</h3><p class="muted">${say('設定手機使用的 WebDAV 地址、帳號與目錄；即時傳輸統一顯示於儀表板。','设置手机使用的 WebDAV 地址、账号与目录；实时传输统一显示于仪表盘。')}</p><button id="backToDashboard" class="btn ghost">${say('查看儀表板','查看仪表盘')}</button></div>
      <div class="share-layout dav-gap">
       <div class="section pad"><h3>${say('已建立帳號','已创建账号')}</h3><p class="muted">${say('每個帳號使用獨立目錄。管理員與 WebDAV 備份帳號分開；舊備份可放入對應目錄，目錄列舉即時更新。','每个账号使用独立目录。管理员与 WebDAV 备份账号分开；旧备份可放入对应目录，目录列表实时更新。')}</p>
        <div class="codebox">${esc(info.storage)}</div><div id="davAccounts" class="dav-gap"></div><button id="davNew" class="btn ghost dav-gap">${say('新增分享','新增共享')}</button>
        <p class="muted">${say('即時列表保留最近 200 筆及進行中的傳輸；重新啟動後，已完成紀錄仍可在事件日誌查閱。百分比僅針對單一檔案或 Range 請求。','实时列表保留最近 200 条及进行中的传输；重启后，已完成记录仍可在事件日志查阅。百分比仅针对单个文件或 Range 请求。')}</p>
       </div>
       <div class="section pad share-form"><h3>${say('連接腳本','连接脚本')}</h3>
        <div class="form-row"><label class="field-label" for="davAddress">webdav_url ${say('（腳本連接地址）','（脚本连接地址）')}</label><div class="form-control-body">
        <input id="davAddress" class="input mono" value="${esc(base)}" spellcheck="false"></div></div>
        ${connection.addresses.length?`<div class="form-row"><label class="field-label" for="davNetwork">${say('偵測到的伺服器 IPv4','检测到的服务器 IPv4')}</label><div class="form-control-body"><select id="davNetwork" class="select"><option value="">${say('保留手動輸入的地址','保留手动输入的地址')}</option>${connection.addresses.map(a=>`<option value="${esc(a.url)}">${esc(a.url)} · ${esc(a.interface)}</option>`).join('')}</select></div></div>`:''}
        <p id="davNetworkHint" class="muted form-note">${connection.loopback_only?say('目前服務僅監聽本機，手機無法連接；請將服務監聽地址改成 0.0.0.0，再重新整理。','当前服务仅监听本机，手机无法连接；请将服务监听地址改为 0.0.0.0，再刷新。'):connection.selection_required?say('此 WebDAV 埠同時監聽多個 IPv4。本機連線無法確定手機使用哪張網卡，請選與手機同網路的地址，或從該區網地址登入以自動確認。','此 WebDAV 端口同时监听多个 IPv4。本机连接无法确定手机使用哪张网卡，请选择与手机同网络的地址，或从该局域网地址登录以自动确认。'):connection.addresses.length?say('地址依 WebDAV 實際監聽埠與目前連線使用的 IPv4 產生。可手動填入自訂網域；外網使用 HTTPS。','地址依 WebDAV 实际监听端口与当前连接使用的 IPv4 生成。可手动填写自定义域名；外网使用 HTTPS。'):say('未取得可用的區網 IPv4，請手動輸入手機可連接的伺服器地址。','未取得可用的局域网 IPv4，请手动输入手机可连接的服务器地址。')}</p>
        <div class="form-row"><label class="field-label" for="davUser">${say('分享名稱／備份帳號','共享名称／备份账号')}</label><div class="form-control-body">
        <input id="davUser" class="input" value="phone" maxlength="48" autocomplete="off"></div></div>
        <div class="form-row"><label class="field-label" for="davDirectory">${say('分享目錄（伺服器實體路徑）','共享目录（服务器实际路径）')}</label><div class="form-control-body">
        <div class="directory-input"><input id="davDirectory" class="input mono" placeholder="${esc(info.storage)}"><button id="davBrowse" class="btn ghost" title="${say('瀏覽伺服器目錄','浏览服务器目录')}">${say('瀏覽…','浏览…')}</button></div>
        <p class="muted">${say('留空使用預設帳號目錄；自訂路徑須已存在，且服務帳號可讀寫。變更不搬移或刪除原備份。','留空使用默认账号目录；自定义路径须已存在，且服务账号可读写。更改不移动或删除原备份。')}</p></div></div>
        <div class="form-note"><label class="check-label"><input id="davAnonymous" type="checkbox"> ${say('匿名分享（腳本不填帳號、密碼）','匿名共享（脚本不填账号、密码）')}</label>
        <p class="muted">${say('開啟後，能連到此匿名網址的人可讀寫這個分享目錄。管理介面仍須登入；關閉匿名分享或停用帳號即停止匿名存取。只有一個匿名分享時可使用根網址；多個分享使用各自網址。','开启后，能连接此匿名地址的人可读写这个共享目录。管理界面仍需登录；关闭匿名共享或停用账号即停止匿名访问。只有一个匿名共享时可使用根地址；多个共享使用各自地址。')}</p></div>
        <div class="form-row"><label class="field-label" for="davPassword">webdav_remote_pass ${say('（密碼；更新帳號時留空可保留）','（密码；更新账号时留空可保留）')}</label><div class="form-control-body">
        <input id="davPassword" class="input" type="password" autocomplete="new-password"></div></div>
        <div class="form-note"><label class="check-label"><input id="davDisabled" type="checkbox"> ${say('停用此帳號（保留備份檔案）','停用此账号（保留备份文件）')}</label>
        <p class="muted tiny">${say('填好後離開欄位或按 Enter 即套用；勾選項目立即套用。','填好后离开字段或按 Enter 即应用；勾选项目立即应用。')}</p>
        <p id="davSaveStatus" class="muted" role="status"></p></div>
        <div class="form-row"><label class="field-label" for="davScriptConfig">${say('貼入腳本設定檔的對應欄位','粘贴到脚本配置文件的对应字段')}</label><div class="form-control-body">
        <p class="muted">${say('確認顯示「已套用」後，複製下方設定取代腳本中同名欄位；remote_stream 保持原設定。','确认显示“已应用”后，复制下方配置替换脚本中同名字段；remote_stream 保持原配置。')}</p>
        <textarea id="davScriptConfig" class="input mono" rows="5" readonly spellcheck="false" aria-label="${say('腳本 WebDAV 設定','脚本 WebDAV 配置')}"></textarea>
        <div class="toolbar dav-gap"><label><input id="davShowPassword" type="checkbox"> ${say('顯示設定中的密碼','显示配置中的密码')}</label><button id="davCopy" class="btn ghost">${say('複製腳本設定（含密碼）','复制脚本配置（含密码）')}</button></div>
        <p id="davConfigHint" class="muted" role="status"></p></div></div>
       </div>

      </div>`;
    const alive=()=>generation===davGeneration&&S.page==='webdav';
    let knownUsers=info.users||[], lastSavedPassword='';
    const snapshot=()=>({username:$('davUser').value.trim(),directory:$('davDirectory').value.trim(),anonymous:$('davAnonymous').checked,disabled:$('davDisabled').checked,password:$('davAnonymous').checked?'':$('davPassword').value});
    function loadAccount(user){
      $('davUser').value=user?.username||'';$('davUser').readOnly=!!user;
      $('davAnonymous').checked=!!user?.anonymous;$('davDisabled').checked=!!user?.disabled;
      $('davDirectory').value=user?.custom_directory?user.directory:'';
      $('davPassword').value='';$('davShowPassword').checked=false;
      lastSavedPassword='';
      try{$('davAddress').value=localStorage.getItem('sb_dav_address:'+user?.username)||base;}catch(_){$('davAddress').value=base;}
    }
    loadAccount(knownUsers[0]);
    const editor=SBDAVEditor.create({initial:knownUsers[0]?snapshot():null,
      save:async(value,previous)=>{
        const body={...value};
        if(previous?.username===value.username&&value.password===lastSavedPassword)body.password='';
        await api('/api/v1/admin/webdav',{method:'POST',body});
        lastSavedPassword=value.password;
        const data=await api('/api/v1/admin/webdav');
        if(!alive())return;
        rootAnonymous=data.root_anonymous_user||'';knownUsers=data.users;drawAccounts();
        if($('davUser').value.trim()===value.username)$('davUser').readOnly=true;
      },
      notify:({saved,busy,error})=>{
        if(!alive())return;
        $('davSaveStatus').textContent=busy?say('正在套用…','正在应用…'):error?(error.name==='TimeoutError'?error.message:say('未套用：','未应用：')+error.message+say('。修正後離開欄位或按 Enter 重試。','。修正后离开字段或按 Enter 重试。')):saved&&!editor.dirty(snapshot())?say('已套用。','已应用。'):say('待填寫完成。','待填写完成。');
        $('davNew').disabled=busy;document.querySelectorAll('.dav-account').forEach(b=>b.disabled=busy);
        updateShareAddress();
      }
    });shareEditor=editor;
    function drawAccounts(){
      $('davAccounts').innerHTML=knownUsers.map(u=>`<button class="btn ghost dav-account" data-user="${esc(u.username)}">${esc(u.username)} · ${u.disabled?say('已停用','已停用'):u.anonymous?say('匿名分享','匿名共享'):say('帳密連接','账号密码连接')}</button>`).join(' ')||empty();
      document.querySelectorAll('.dav-account').forEach(b=>b.onclick=()=>{if(editor.busy())return;loadAccount(knownUsers.find(u=>u.username===b.dataset.user));editor.reset(snapshot());updateShareAddress();});
    }
    function commit(){
      const value=snapshot(),existing=knownUsers.some(u=>u.username===value.username);
      if(!/^[a-z0-9_-]{1,48}$/.test(value.username)||(!existing&&!value.anonymous&&!value.password.trim())){
        $('davSaveStatus').textContent=say('待填寫：有效分享名稱，以及密碼或匿名分享。','待填写：有效共享名称，以及密码或匿名共享。');updateConfig();return;
      }
      if(!editor.dirty(value))return;editor.submit(value);
    }
    $('davNew').onclick=()=>{if(editor.busy())return;loadAccount(null);editor.reset(null);updateShareAddress();$('davUser').focus();};
    function scriptConfig(){return SBWebDAV.config($('davAddress').value.trim(),$('davUser').value.trim(),$('davPassword').value,$('davAnonymous').checked,rootAnonymous);}
    function updateConfig(){
      const hint=$('davConfigHint');const anonymous=$('davAnonymous').checked;
      $('davPassword').disabled=anonymous;$('davShowPassword').disabled=anonymous;
      $('davCopy').textContent=anonymous?say('複製匿名腳本設定','复制匿名脚本配置'):say('複製腳本設定（含密碼）','复制脚本配置（含密码）');
      try{
        const config=scriptConfig();
        $('davScriptConfig').value=(anonymous||$('davShowPassword').checked)?config:config.split('\n').slice(0,3).join('\n')+'\n# webdav_remote_pass: '+say('密碼已隱藏；下方複製按鈕會包含完整密碼','密码已隐藏；下方复制按钮会包含完整密码');
        $('davCopy').disabled=$('davDisabled').checked||editor.busy()||editor.dirty(snapshot());
        hint.textContent=$('davDisabled').checked?say('此帳號已勾選停用，請取消停用並確認已套用後再連接。','此账号已勾选停用，请取消停用并确认已应用后再连接。'):anonymous?say('匿名設定的帳號、密碼會留空；確認上方顯示「已套用」後即可使用。','匿名配置的账号、密码会留空；确认上方显示“已应用”后即可使用。'):say('複製時會包含本次輸入的完整密碼。','复制时会包含本次输入的完整密码。');
      }catch(e){
        $('davCopy').disabled=true;
        const messages={loopback:say('手機不能使用本機或全部網卡的監聽地址，請選擇可連接的 IPv4 或輸入網域。','手机不能使用本机或全部网卡的监听地址，请选择可连接的 IPv4 或输入域名。'),password:say('請輸入此備份帳號的密碼，才能產生完整設定。既有密碼無法反查；輸入密碼並離開欄位後會自動套用。','请输入此备份账号的密码，才能生成完整配置。已有密码无法反查；输入密码并离开字段后会自动应用。'),url:say('請輸入有效的 HTTP／HTTPS WebDAV 地址。','请输入有效的 HTTP／HTTPS WebDAV 地址。'),user:say('請填入有效的備份帳號。','请填写有效的备份账号。'),line:say('腳本設定欄位不可包含換行或 NUL 字元。','脚本配置字段不能包含换行或 NUL 字符。')};
        hint.textContent=messages[e.message]||e.message;
        $('davScriptConfig').value=e.message==='password'?SBWebDAV.config($('davAddress').value.trim(),$('davUser').value.trim(),'PASSWORD').split('\n').slice(0,3).join('\n')+'\n# '+say('請在上方輸入備份帳號密碼','请在上方输入备份账号密码'):'';
      }
    }
    function updateShareAddress(){$('davAddress').value=SBWebDAV.shareAddress($('davAddress').value.trim(),$('davUser').value.trim(),$('davAnonymous').checked,rootAnonymous);updateConfig();}
    ['davUser','davPassword','davDirectory'].forEach(id=>{
      $(id).addEventListener('input',()=>{updateShareAddress();$('davSaveStatus').textContent=say('編輯中，離開欄位或按 Enter 套用。','编辑中，离开字段或按 Enter 应用。');});
      $(id).addEventListener('change',commit);
      $(id).addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();commit();}});
    });
    function rememberAddress(){updateShareAddress();try{SBWebDAV.config($('davAddress').value.trim(),$('davUser').value.trim(),'x',$('davAnonymous').checked,rootAnonymous);localStorage.setItem('sb_dav_address:'+$('davUser').value.trim(),$('davAddress').value.trim());}catch(_){}}
    $('davAddress').addEventListener('change',rememberAddress);
    $('davAddress').addEventListener('input',updateConfig);
    $('davAnonymous').onchange=()=>{updateShareAddress();commit();};$('davDisabled').onchange=()=>{updateConfig();commit();};$('davShowPassword').onchange=updateConfig;
    if($('davNetwork')){
      $('davNetwork').value=connection.addresses.some(a=>a.url===base)?base:'';
      $('davNetwork').onchange=()=>{if($('davNetwork').value){$('davAddress').value=$('davNetwork').value;rememberAddress();}};
      $('davAddress').addEventListener('input',()=>{$('davNetwork').value=connection.addresses.some(a=>a.url===$('davAddress').value)?$('davAddress').value:'';});
    }
    updateShareAddress();
    $('davBrowse').onclick=()=>browseDirectory(generation);
    $('davCopy').onclick=async()=>{
      const value=scriptConfig();
      try{if(navigator.clipboard&&window.isSecureContext){await navigator.clipboard.writeText(value);}else{const box=document.createElement('textarea');box.value=value;document.body.appendChild(box);box.select();const ok=document.execCommand('copy');box.remove();if(!ok)throw new Error('clipboard unavailable');}toast(t('copied'));}catch(e){$('davSaveStatus').textContent=say('無法存取剪貼簿，請手動複製欄位內容。','无法访问剪贴板，请手动复制字段内容。');}
    };
    drawAccounts();editor.reset(knownUsers[0]?snapshot():null);
    $('backToDashboard').onclick=()=>setPage('dashboard');
  }

  let lastDebugReport=0;
  function reportUIError(error){
    if($('appView').classList.contains('hidden')||Date.now()-lastDebugReport<10000)return;
    lastDebugReport=Date.now();
    const body={page:S.page,name:String(error?.name||'Error').slice(0,48),message:String(error?.message||error).replace(/\b(?:https?|file):\/\/\S+/g,'[URL]').slice(0,512),stack:String(error?.stack||'').slice(0,8192)};
    void api('/api/v1/admin/diagnostics/client-error',{method:'POST',body}).catch(()=>{});
  }
  window.addEventListener('error',e=>reportUIError(e.error||new Error(e.message)));
  window.addEventListener('unhandledrejection',e=>reportUIError(e.reason));

  async function renderPage(){
    if(!canLeave())return;shareEditor=null;stopDavMonitor();const generation=++davGeneration;
    try {
      if(S.page==='webdav') await renderWebDAV(); else if(S.page==='dashboard') await renderDashboard(); else if(S.page==='profiles') await renderProfiles(); else if(S.page==='events') await renderEvents(); else if(S.page==='settings') await renderSettings();
    } catch(e) { if(generation!==davGeneration)return;if(e.status===401){logout();return;} reportUIError(e);$('content').innerHTML=`<div class="alert error">${esc(t('request_failed'))}: ${esc(e.message)}</div><a class="btn ghost" href="/api/v1/admin/diagnostics/export" download>${S.locale==='zh-CN'?'下载服务器调试包':'下載伺服器除錯包'}</a>`; }
  }

  $('loginBtn').onclick=login; $('adminPassword').addEventListener('keydown',e=>{if(e.key==='Enter')login();}); $('logoutBtn').onclick=logout;
  document.querySelectorAll('.locale-btn').forEach(b=>b.onclick=()=>setLocale(b.dataset.locale));
  $('localeSelect').onchange=e=>setLocale(e.target.value);
  document.querySelectorAll('.nav-item').forEach(b=>b.onclick=()=>setPage(b.dataset.page));
  $('refreshBtn').onclick=async()=>{await renderPage();toast(t('refresh_ok'));};
  window.addEventListener('beforeunload',e=>{if(shareEditor?.busy()){e.preventDefault();e.returnValue='';}});
  applyI18n();
  initializeLogin();
})();
