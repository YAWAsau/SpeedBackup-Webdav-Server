const {chromium}=require('playwright'),fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{spawn,spawnSync}=require('node:child_process'),net=require('node:net');
const src=path.resolve(__dirname,'..'),out=path.join(src,'dist','save-status-evidence');fs.mkdirSync(out,{recursive:true});
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

 await page.locator('[data-page=profiles]').click();await page.locator('#configureShares').click();
 const status=page.locator('#davSaveBanner');
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner')?.dataset.state==='pending');
 await page.locator('#davUser').fill('uitest');
 let mode='slow',release;await page.route('**/api/v1/admin/webdav',async route=>{
  if(route.request().method()!=='POST')return route.continue();
  if(mode==='fail')return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({error:'test save rejected'})});
  if(mode==='slow')await new Promise(r=>release=r);
  return route.continue();
 });
 await page.locator('#davAnonymous').check();
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='saving');
 await page.waitForFunction(()=>document.querySelector('#davSaveStatus').textContent.includes('儲存中'));
 await page.waitForTimeout(100);release();mode='normal';
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='saved');
 await status.scrollIntoViewIfNeeded();await page.screenshot({path:path.join(out,'saved.png'),fullPage:true});
 mode='fail';await page.locator('#davDisabled').check();
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='error');
 assert.match(await page.locator('#davSaveStatus').innerText(),/儲存失敗/);
 await page.screenshot({path:path.join(out,'failed.png'),fullPage:true});
 mode='normal';await page.locator('#davDisabled').uncheck();
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='saved');
 await page.setViewportSize({width:393,height:950});await page.locator('#davPassword').scrollIntoViewIfNeeded();
 const bounds=await status.boundingBox();assert(bounds.y>=0&&bounds.y+bounds.height<=950,'status remains visible while editing');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 await page.screenshot({path:path.join(out,'mobile.png')});assert.deepEqual(errors,[]);
 console.log('PASS: pending, in-flight, server acknowledgement, failure, revert and sticky mobile status');
 }finally{await browser?.close();server.kill();await new Promise(r=>server.exitCode!==null?r():server.once('exit',r));}
})().catch(e=>{console.error(e);process.exitCode=1});
