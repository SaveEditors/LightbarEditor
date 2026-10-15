// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors
// Exercise the actual embedded UI functions without a browser or console assets.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const html=readFileSync(new URL('./ui/index.html',import.meta.url),'utf8');
function between(start,end){const a=html.indexOf(start);assert(a>=0,start);const b=html.indexOf(end,a+start.length);assert(b>a,end);return html.slice(a,b);}
function fn(name){const start=html.indexOf(`function ${name}(`);assert(start>=0,name);for(let end=html.indexOf('\n',start);end>=0;end=html.indexOf('\n',end+1)){const source=html.slice(start,end);try{new vm.Script(source);return source;}catch{}}throw new Error(`Cannot extract ${name}`);}
const noop=()=>{};
const nodes=new Map();
const context=vm.createContext({
 console, SETTINGS:{prefs:{}}, followThemeParticles:true,currentThemeParticles:null,partState:null,
 M:null,S:null,R:{fx:[null,null],adj:[{},{}]},anim:{half:0,model:'bright',playing:true},
 half:0,selKey:-1,hist:[],histI:0,
 $:selector=>{if(!nodes.has(selector))nodes.set(selector,{checked:true,classList:{toggle:noop}});return nodes.get(selector);},
 $$:()=>[],saveSettings:noop,refreshRamp:noop,syncParticleUI:noop,syncStripUI:noop,
 syncRampUI:noop,renderKeyEditor:noop,draw3d:noop,drawTimeline:noop,drawRamp:noop,
 updateFxBar:noop,updateGnfBar:noop,togglePlay:noop,toast:noop,esc:s=>s,
 rampAt:()=>[1,1,1],stripPalette:()=>['#ffffff'],loadMyPresets:noop,
 prompt:()=> 'Saved custom effect',api:async(route,body)=>{context.savedRequest={route,body};return {};},
 fxChanged:()=>context.commit(),
});
const run=source=>vm.runInContext(source,context);
run(html.split('\n').find(line=>line.startsWith('const rgb2hex =')));
run(fn('hsl2rgb'));
run(between('const K =','// ---------- side panel tabs ----------'));
run(between('// effect presets that also recolour the particles','function applyPreset(p)'));
for(const name of ['defaultAdj','setThemeParticleMode','customizeParticles','setParticles','partFromFile','applyPreset','ensureFx','resetHistory','commit','restoreSnap','undo','redo','rampChanged'])run(fn(name));
run(html.split('\n').find(line=>line.startsWith('const snap =')));
run(between("$('#partFollowTheme').onchange", "$('#partAdd').onclick"));
run(between("$('#savePreset').onclick",'\nloadMyPresets();'));
const themes=run('BUILTIN.filter(p=>!p.original)');
assert.equal(themes.length,21);
for(const p of themes){
 assert(p.particles.cols.length,`${p.name}: missing palette`);
 context.applyPreset(p);
 assert.equal(context.followThemeParticles,true,`${p.name}: automatic matching disabled`);
 assert.deepEqual(JSON.parse(JSON.stringify(context.partFromFile().cols)),JSON.parse(JSON.stringify(p.particles.cols)));
}
const matrix=themes.find(p=>p.name==='Matrix'),police=themes.find(p=>p.name==='Police');
context.setParticles(['#123456','#abcdef'],'smooth',.65);
assert.equal(context.followThemeParticles,false);
const custom=JSON.stringify(context.R.fx[1]);
context.R.adj[1]={hue:27,sat:80,bri:75};
const adjustments=JSON.stringify(context.R.adj[1]);
context.applyPreset(police);
assert.equal(JSON.stringify(context.R.fx[1]),custom,'theme overwrote custom particle colors/mix/brightness');
assert.equal(JSON.stringify(context.R.adj[1]),adjustments,'theme overwrote particle row adjustments');
context.resetHistory();
nodes.get('#partFollowTheme').onchange({target:{checked:true}});
assert.equal(context.followThemeParticles,true);
assert.equal(JSON.stringify(context.partFromFile().cols),JSON.stringify(police.particles.cols));
context.undo();
assert.equal(context.followThemeParticles,false);
assert.equal(JSON.stringify(context.R.fx[1]),custom);
assert.equal(JSON.stringify(context.R.adj[1]),adjustments);
context.redo();
assert.equal(context.followThemeParticles,true);
context.applyPreset(matrix);
context.half=1;context.R.adj[1].hue=90;context.rampChanged();
assert.equal(context.followThemeParticles,false,'manual particle-row edit did not preserve customization');
context.setParticles(['#112233','#44ee55'],'step',.8);
context.anim.half=1; // Save from the particle tab: strip keys must still use strip rows.
context.R.fx[0]={interp:'smooth',keys:[{t:0,c:'#fedcba',b:1}]};
await nodes.get('#savePreset').onclick();
assert.equal(context.savedRequest.route,'/api/presets/save');
assert.equal(context.savedRequest.body.preset.keys[0].c,'#fedcba');
assert.equal(context.savedRequest.body.preset.particleMode,'custom');
assert.equal(JSON.stringify(context.savedRequest.body.preset.particles.cols),JSON.stringify(['#112233','#44ee55']));
context.setThemeParticleMode(true);
context.applyPreset(context.savedRequest.body.preset);
assert.equal(context.followThemeParticles,false,'saved custom effect lost customization mode');
assert.equal(JSON.stringify(context.partFromFile().cols),JSON.stringify(['#112233','#44ee55']));
console.log('PASS: 21 theme defaults; custom palette/mix/brightness and ramp adjustment preservation; resume/undo/redo; saved particle palettes and correct strip keys.');
