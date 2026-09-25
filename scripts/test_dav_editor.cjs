const assert=require('node:assert/strict');
const {create}=require('../internal/sbserver/web/dav_editor.js');
(async()=>{
 const writes=[],resolvers=[];let concurrent=0,max=0,last;
 const editor=create({initial:{path:'old'},save:async value=>{writes.push(value.path);max=Math.max(max,++concurrent);await new Promise(resolve=>resolvers.push(resolve));concurrent--;},notify:state=>last=state});
 editor.submit({path:'first'});editor.submit({path:'second'});editor.submit({path:'final'});
 assert.deepEqual(writes,['first']);assert.equal(editor.busy(),true);assert.equal(editor.dirty({path:'final'}),true);
 resolvers.shift()();await new Promise(setImmediate);assert.deepEqual(writes,['first','final']);
 resolvers.shift()();await editor.settled();assert.equal(max,1);assert.equal(editor.dirty({path:'final'}),false);assert.equal(last.busy,false);
 editor.submit({path:'final'});await editor.settled();assert.equal(writes.length,2);
 const failed=create({initial:{path:'old'},save:async()=>{throw new Error('directory inaccessible');},notify:state=>last=state});
 failed.submit({path:'bad'});await failed.settled();assert.equal(failed.dirty({path:'bad'}),true);assert.deepEqual(last.saved,{path:'old'});assert.match(last.error.message,/inaccessible/);
 failed.reset({path:'other'});assert.equal(failed.dirty({path:'other'}),false);
 console.log('PASS: serialized writes, coalescing, settled state, duplicate suppression, failed-save preservation');
})().catch(e=>{console.error(e);process.exitCode=1;});
