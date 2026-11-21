const {chromium}=require('playwright'),fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{spawn,spawnSync}=require('node:child_process'),net=require('node:net');
const src=path.resolve(__dirname,'..'),out=path.join(src,'dist','accounts-evidence');fs.mkdirSync(out,{recursive:true});
(async()=>{
 const data=fs.mkdtempSync(path.join(out,'fixture-')),exe=path.join(src,'dist','windows-amd64','speedbackup-server.exe');
 const port=await new Promise(resolve=>{const s=net.createServer();s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p));});});
 for(const args of [['init','--root',data],['admin-reset','--root',data,'--username','uitest','--password-stdin']]){const r=spawnSync(exe,args,{input:'test-password\n',windowsHide:true});assert.equal(r.status,0,r.stderr.toString());}
 const server=spawn(exe,['serve','--root',data,'--listen',`127.0.0.1:${port}`],{windowsHide:true,stdio:'ignore'});let browser;
 try{
 const origin=`http://127.0.0.1:${port}`;for(let i=0;i<100;i++){try{if((await fetch(origin+'/api/v1/auth/status')).ok)break;}catch{}await new Promise(r=>setTimeout(r,100));}
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe'});
 const page=await browser.newPage({viewport:{width:1440,height:1050}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(origin+'/web/admin');await page.locator('#adminUser').fill('uitest');await page.locator('#adminPassword').fill('test-password');await page.locator('#loginBtn').click();await page.locator('[data-page=settings]').click();
 const seed=await page.evaluate(async()=>{const r=await fetch('/api/v1/admin/webdav',{method:'POST',headers:{'Content-Type':'application/json','X-SB-Admin':'1'},body:JSON.stringify({username:'root',password:'root-fixture-password'})});return r.status;});assert.equal(seed,200);
 await page.locator('[data-page=profiles]').click();await page.locator('#configureShares').click();
 await page.waitForFunction(()=>document.querySelector('#davUser')?.value==='root');
 assert.equal(await page.locator('#davUser').evaluate(e=>e.readOnly),true);
 assert.match(await page.locator('#davUserHint').innerText(),/新增帳號/);
 await page.locator('#davNewAccount').click();
 assert.equal(await page.locator('#davUser').evaluate(e=>e.readOnly),false);
 await page.locator('#davUser').fill('_bad');await page.locator('#davUser').press('Enter');assert.match(await page.locator('#davSaveStatus').innerText(),/開頭須/);
 await page.locator('#davUser').fill('root');await page.locator('#davUser').press('Enter');assert.match(await page.locator('#davSaveStatus').innerText(),/已存在/);
 await page.locator('#davUser').fill('phone_2');await page.locator('#davPassword').fill('phone-fixture-password');await page.locator('#davPassword').press('Enter');
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='saved');
 assert.equal(await page.locator('#davUser').inputValue(),'phone_2');
 const auth=(u,p)=>({'Authorization':'Basic '+Buffer.from(u+':'+p).toString('base64')});
 let r=await fetch(origin+'/account-test.txt',{method:'PUT',headers:auth('phone_2','phone-fixture-password'),body:'phone-owned'});assert.equal(r.status,201);
 r=await fetch(origin+'/account-test.txt',{headers:auth('phone_2','phone-fixture-password')});assert.equal(r.status,200);assert.equal(await r.text(),'phone-owned');
 r=await fetch(origin+'/account-test.txt',{headers:auth('root','root-fixture-password')});assert.equal(r.status,404,'root account retained and isolated');
 r=await fetch(origin+'/account-test.txt',{headers:auth('phone_2','wrong')});assert.equal(r.status,401);
 await page.reload();await page.locator('[data-page=profiles]').click();await page.locator('#configureShares').click();await page.locator('.dav-account[data-user="phone_2"]').click();assert.equal(await page.locator('#davUser').inputValue(),'phone_2');

 // Rename root through the actual UI; existing files and password survive.
 r=await fetch(origin+'/root-backup.zst',{method:'PUT',headers:auth('root','root-fixture-password'),body:'keep-root-backup'});assert.equal(r.status,201);
 await page.locator('.dav-account[data-user="root"]').click();
 await page.locator('#davRename').click();await page.locator('#davRenameName').fill('phone_2');await page.locator('#davRenameApply').click();
 await page.waitForFunction(()=>document.querySelector('#davRenameStatus').textContent.includes('已存在'));
 await page.locator('#davRenameName').fill('family');await page.locator('#davRenameApply').click();
 await page.waitForFunction(()=>document.querySelector('#davUser').value==='family');
 assert.match(await page.locator('#davSaveStatus').innerText(),/更名完成/);
 r=await fetch(origin+'/root-backup.zst',{headers:auth('root','root-fixture-password')});assert.equal(r.status,401);
 r=await fetch(origin+'/root-backup.zst',{headers:auth('family','root-fixture-password')});assert.equal(r.status,200);assert.equal(await r.text(),'keep-root-backup');
 const sharedDir=await page.locator('#davDirectory').inputValue();assert(sharedDir.endsWith('root'));
 await page.locator('#davSharedDirectory').check();await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='saved');
 await page.locator('#davNewAccount').click();await page.locator('#davUser').fill('family2');await page.locator('#davSharedDirectory').check();await page.locator('#davDirectory').fill(sharedDir);await page.locator('#davPassword').fill('family2-password');await page.locator('#davPassword').press('Enter');
 await page.waitForFunction(()=>document.querySelector('#davSaveBanner').dataset.state==='saved');
 r=await fetch(origin+'/root-backup.zst',{headers:auth('family2','family2-password')});assert.equal(r.status,200);assert.equal(await r.text(),'keep-root-backup');
 assert.equal(fs.readFileSync(path.join(data,'webdav','root','root-backup.zst'),'utf8'),'keep-root-backup');
 await page.reload();await page.locator('[data-page=profiles]').click();await page.locator('#configureShares').click();await page.locator('.dav-account[data-user="family"]').click();assert.equal(await page.locator('#davUser').inputValue(),'family');assert(await page.locator('#davSharedDirectory').isChecked());
 console.log('PASS: root rename, duplicate rename rejection, original password/files, revoked old name, opt-in shared directory and persisted UI settings');
 await page.setViewportSize({width:393,height:950});await page.locator('#davNewAccount').scrollIntoViewIfNeeded();assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 await page.screenshot({path:path.join(out,'new-account-mobile.png')});assert.deepEqual(errors,[]);
 console.log('PASS: root read-only explanation, adjacent new-account entry, valid non-root creation, invalid/duplicate rejection, persistence, authenticated PUT/GET, isolation and wrong-password rejection');
 }finally{await browser?.close();server.kill();await new Promise(r=>server.exitCode!==null?r():server.once('exit',r));}
})().catch(e=>{console.error(e);process.exitCode=1});
