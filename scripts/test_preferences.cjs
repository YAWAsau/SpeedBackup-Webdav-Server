const assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const code=fs.readFileSync(require.resolve('../internal/sbserver/web/preferences.js'),'utf8');
function load(storage){const ctx={document:{documentElement:{dataset:{}}},localStorage:storage};vm.createContext(ctx);vm.runInContext(code,ctx);return ctx;}
const data=new Map([['sb_theme','black']]);const storage={getItem:k=>data.get(k),setItem:(k,v)=>data.set(k,v)};
let ctx=load(storage),p=ctx.SBPreferences;
assert.equal(p.get(),'black');assert.equal(p.update({accent:'teal',density:'compact',motion:'reduce'}),true);
ctx=load(storage);p=ctx.SBPreferences;assert.equal(ctx.document.documentElement.dataset.accent,'teal');assert.equal(p.all().density,'compact');
p.update({theme:'url(evil)',accent:'invalid'});assert.equal(p.get(),'system');assert.equal(p.all().accent,'blue');
const copy=p.all();copy.theme='dark';assert.equal(p.get(),'system');
data.set('sb_appearance','{broken');assert.equal(load(storage).SBPreferences.get(),'system');
data.set('sb_appearance','null');assert.equal(load(storage).SBPreferences.get(),'system');
ctx=load({getItem(){throw Error('denied');},setItem(){throw Error('denied');}});p=ctx.SBPreferences;
assert.equal(p.update({theme:'dark'}),false);assert.equal(ctx.document.documentElement.dataset.theme,'dark');
p.reset();assert.equal(p.get(),'system');
console.log('PASS: appearance migration, persistence, validation, malformed/blocked storage and reset');
