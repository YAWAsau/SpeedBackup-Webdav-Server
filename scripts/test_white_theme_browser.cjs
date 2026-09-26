// Requires Playwright and Chromium. CHROME_PATH may select an installed Chrome.
// Compare rendered pixels: computed CSS alone cannot detect automatic darkening.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const {chromium} = require('playwright');
const web = path.join(__dirname, '../internal/sbserver/web');
const server = http.createServer((req, res) => {
  if (req.url === '/styles.css' || req.url === '/preferences.js') {
    res.setHeader('Content-Type', req.url.endsWith('.css') ? 'text/css' : 'text/javascript');
    res.end(fs.readFileSync(path.join(web, req.url.slice(1))));
  } else {
    res.setHeader('Content-Type', 'text/html');
    res.end('<!doctype html><script src="/preferences.js"></script><link rel="stylesheet" href="/styles.css"><body></body>');
  }
});
(async () => {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const samples = [];
  try {
    for (const forced of [false, true]) {
      const browser = await chromium.launch({headless:true,
        ...(process.env.CHROME_PATH ? {executablePath:process.env.CHROME_PATH} : {}),
        args:forced ? ['--enable-features=WebContentsForceDark'] : []});
      try {
        const page = await browser.newPage({colorScheme:'dark', viewport:{width:400,height:300}});
        await page.goto(`http://127.0.0.1:${server.address().port}`);
        for (const theme of ['dark', 'black', 'white']) {
          await page.evaluate(value => SBPreferences.update({theme:value}), theme);
          assert.equal(await page.evaluate(() => document.documentElement.dataset.theme), theme);
        }
        await page.reload();
        assert.equal(await page.evaluate(() => SBPreferences.get()), 'white');
        samples.push(await page.screenshot({clip:{x:200,y:200,width:20,height:20}}));
      } finally { await browser.close(); }
    }
    assert.deepEqual(samples[1], samples[0], 'Explicit white was recolored by Chrome Auto Dark Mode');
    console.log('PASS: white renders identically with Auto Dark Mode on/off and persists across reload');
  } finally { await new Promise(resolve => server.close(resolve)); }
})().catch(error => { console.error(error); process.exitCode=1; });
