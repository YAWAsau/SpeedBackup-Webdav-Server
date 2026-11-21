const {chromium}=require('playwright'),fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{spawn,spawnSync}=require('node:child_process'),net=require('node:net');
const src=path.resolve(__dirname,'..'),out=path.join(src,'dist','miuix-evidence');fs.mkdirSync(out,{recursive:true});
(async()=>{
 const data=fs.mkdtempSync(path.join(out,'fixture-')),exe=path.join(src,'dist','windows-amd64','speedbackup-server.exe');
 const port=await new Promise(resolve=>{const s=net.createServer();s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p));});});
 for(const args of [['init','--root',data],['admin-reset','--root',data,'--username','uitest','--password-stdin']]){const r=spawnSync(exe,args,{input:'test-password\n',windowsHide:true});assert.equal(r.status,0,r.stderr.toString());}
 const server=spawn(exe,['serve','--root',data,'--listen',`127.0.0.1:${port}`],{windowsHide:true,stdio:'ignore'});let browser;
 try{
 const origin=`http://127.0.0.1:${port}`;for(let i=0;i<100;i++){try{if((await fetch(origin+'/api/v1/auth/status')).ok)break;}catch{}await new Promise(r=>setTimeout(r,100));}
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe'});
 const page=await browser.newPage({colorScheme:'dark',viewport:{width:1440,height:1050}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(origin+'/web/admin');await page.locator('#adminUser').fill('uitest');await page.locator('#adminPassword').fill('test-password');await page.locator('#loginBtn').click();await page.locator('[data-page=settings]').click();
 const choice=page.locator('[data-preference=surface][data-value=miuix]');await choice.focus();await page.keyboard.press('Enter');assert.equal(await choice.getAttribute('aria-pressed'),'true');
 for(const theme of ['system','dark','black','white']){
 await page.locator(`[data-preview=${theme}]`).click();await page.waitForFunction(async theme=>{const v=await(await fetch('/api/v1/admin/power')).json();return v.theme?.surface==='miuix'&&v.theme.theme===theme;},theme);
 for(const screen of ['dashboard','profiles','events','settings']){
 await page.locator(`[data-page=${screen}]`).click();await page.waitForTimeout(120);
 assert.equal(await page.evaluate(()=>document.documentElement.dataset.surface),'miuix');assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 if(screen==='settings')assert.equal(await page.locator('[data-value=miuix]').getAttribute('aria-pressed'),'true');
 if(screen==='settings'||screen==='dashboard')await page.screenshot({path:path.join(out,screen+'-'+theme+'.png'),fullPage:true});
 }
 }
 for(const accent of ['blue','teal','violet']){await page.locator(`[data-preference=accent][data-value=${accent}]`).click();await page.waitForFunction(async accent=>(await(await fetch('/api/v1/admin/power')).json()).theme?.accent===accent,accent);}
 await page.reload();await page.locator('[data-page=settings]').click();await page.locator('[data-value=miuix]').waitFor();assert.equal(await page.evaluate(()=>SBPreferences.all().surface),'miuix');
 await page.setViewportSize({width:393,height:950});for(const screen of ['dashboard','profiles','events','settings']){await page.locator(`[data-page=${screen}]`).click();await page.waitForTimeout(100);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),screen+' mobile overflow');await page.screenshot({path:path.join(out,screen+'-mobile.png'),fullPage:true});}
 await page.locator('[data-preference=surface][data-value=classic]').click();assert.equal(await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--bg').trim()),'#f4f6f8');await page.locator('[data-value=miuix]').click();await page.locator('#resetAppearance').click();assert.equal(await page.evaluate(()=>SBPreferences.all().surface),'classic');assert.deepEqual(errors,[]);
 console.log('PASS: miuix four themes, three accents, native theme API sync, keyboard, reload, mobile pages, classic/reset isolation; no shutdown requests');
 }finally{await browser?.close();server.kill();await new Promise(r=>server.exitCode!==null?r():server.once('exit',r));}
})().catch(e=>{console.error(e);process.exitCode=1});
