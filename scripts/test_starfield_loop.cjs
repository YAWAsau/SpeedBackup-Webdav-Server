const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
let strokes=[],stars=[],stack=[],arc=null;
const ctx={globalAlpha:1,tx:0,ty:0,save(){stack.push([this.globalAlpha,this.tx,this.ty])},restore(){[this.globalAlpha,this.tx,this.ty]=stack.pop()},translate(x,y){this.tx+=x;this.ty+=y},scale(){},rotate(){},setTransform(){},clearRect(){},fillRect(){},strokeRect(){},beginPath(){arc=null},moveTo(){},lineTo(){},arc(x,y){arc={x,y}},fill(){if(arc)stars.push({...arc,a:this.globalAlpha})},stroke(){strokes.push({x:this.tx,y:this.ty,a:this.globalAlpha})},createRadialGradient(){return {addColorStop(){}}},createLinearGradient(){return {addColorStop(){}}}};
const canvas={setAttribute(){},getContext(){return ctx}};
const sandbox={Math:Object.assign(Object.create(Math),{random:()=>.5}),document:{createElement:()=>canvas,body:{prepend(){}},addEventListener(){}},window:{innerWidth:1440,innerHeight:900,devicePixelRatio:1,addEventListener(){}},matchMedia:()=>({matches:false,addEventListener(){}}),SBPreferences:{}};
let code=fs.readFileSync(require.resolve('../internal/sbserver/web/starfield.js'),'utf8').replace('SBPreferences.subscribe(sync);sync();','globalThis.drawAt=value=>{phase=value;draw();};size();');
vm.createContext(sandbox);vm.runInContext(code,sandbox);
function frame(t){strokes=[];stars=[];sandbox.drawAt(t);return {meteors:strokes.slice(30,38),icons:strokes.slice(38),stars};}
// Each meteor crosses its cycle only while fully faded; former vertical-wrap
// times are also sampled to detect an on-screen teleport inside the flight.
for(let m=0;m<8;m++){
 for(const travel of [0,1,((1-(m*.31)%1)/.5)%1]){
  const time=(travel-m/8+2)/.28,a=frame(time-.00001).meteors[m],b=frame(time+.00001).meteors[m];
  assert(Math.hypot(a.x-b.x,a.y-b.y)<1 || Math.max(a.a,b.a)<1e-6,`meteor ${m} teleported while visible`);
 }
}
const starTime=(900+80)*.5/(.9*22);
for(const t of [starTime-.00001,starTime+.00001])assert(frame(t).stars.every(s=>s.a===0),'star wraps invisibly');
for(let i=0;i<5;i++){
 const t=(1020-900*(.12+i*.19))/(12+i*1.2);
 for(const d of [-.00001,.00001])assert.equal(frame(t+d).icons[i].a,0,'icon wraps invisibly');
}
console.log('PASS: meteor cycle and former vertical-wrap boundaries; stars/icons reset fully invisible');
