'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(__dirname+'/v66_cloud_refusal_gate.js','utf8');
const phrases = [
 '被你问住了，看来要更努力学习了',
 '这个问题我暂时还回答不上，需要再学习一下',
 '这可把我难住了，看来要更努力学习了',
 '这个问题把我难住了，我会继续努力的',
 '这个我暂时还回答不上诶，我要再学习学习',
 '这个问题我还在学习中，先聊点别的吧',
 '被难住了诶，看来我还要再学习一下',
 '对不起，我还在学习中!'
];
function simulate(filter, choice){
 let processCalled=0, batchCalled=0, originalBatch, processOriginal=[];
 const logs=[];
 const cap={implementation:null,returnType:{className:'boolean'},call(_self,ins){processCalled++;processOriginal.push(ins);return false;}};
 const op={implementation:null,returnType:{className:'void'},call(_self,id,list){batchCalled++;originalBatch={id,list};}};
 const clazz = n => {
  if(n.endsWith('InstructionCapabilityImpl')) return {process:{overload:()=>cap}};
  if(n.endsWith('OperationManager')) return {addInstructions:{overload:()=>op}};
  if(n==='java.util.ArrayList') return {$new:()=>({v:[],add(x){this.v.push(x);},size(){return this.v.length;}})};
  return {};
 };
 const J={available:true,perform:f=>f(),use:clazz,cast:(x)=>x};
 const ctx={Java:J,console:{log:s=>logs.push(String(s))},Object,Array,String,RegExp,JSON,Number,Boolean,Math};
 vm.runInNewContext(filter?source.replace('var V66_EXPERIMENTAL_FILTER = false;','var V66_EXPERIMENTAL_FILTER = true;'):source,ctx);
 const id='private-dialog';
 const opt=x=>({isPresent:()=>Boolean(x),get:()=>x});
 const item=(name,payload={})=>({getFullName:()=>name,getDialogId:()=>opt(id),getPayload:()=>payload});
 const asr=item('SpeechRecognizer.RecognizeResult',{isFinal:()=>true,getResults:()=>({size:()=>1,get:()=>({getText:()=> '请解释一下延迟器'})})});
 const start=item('Nlp.StartAnswer');
 const toast=item('Template.Toast',{getText:()=>choice.toast,getQuery:()=>opt('请解释一下延迟器')});
 const speak=item('SpeechSynthesizer.Speak',{getText:()=>choice.toast});
 const finish=item('Nlp.FinishAnswer');
 const batch={v:[toast,speak],size(){return this.v.length;},get(i){return this.v[i];}};
 const all=[asr,start,toast,speak,finish];
 if(choice.conflict) all.push(item('PlaybackController.Pause'));
 if(!choice.noAsr) for(const x of all) cap.implementation.call({},x);
 else for(const x of all.slice(1)) cap.implementation.call({},x);
 const receiver={};
 op.implementation.call(receiver,id,batch);
 assert.equal(batchCalled,1);
 assert.equal(processCalled,choice.noAsr?all.length-1:all.length);
 assert.equal(originalBatch.id,id);
 const expected=phrases.includes(choice.toast)&&!choice.conflict&&!choice.noAsr;
 const decision=logs.some(x=>x.includes('V66_BATCH_OBS')&&x.includes('"refusal":true'));
 assert.equal(decision,expected,JSON.stringify(choice));
 assert.equal(originalBatch.list===batch,!expected||!filter);
 assert.equal(batch.v.length,2,'original OEM input list not mutated');
 if(expected && filter) assert.equal(originalBatch.list.v.length,0);
 assert.ok(!logs.join('\n').includes(id),'never log a full dialog ID');
 return logs;
}
for(const text of phrases){simulate(false,{toast:text});simulate(true,{toast:text});}
for(const text of ['好的','正在降低晾衣机。','哎呀，你说什么了，我没有听清楚！']) {
 simulate(false,{toast:text});simulate(true,{toast:text});
}
simulate(true,{toast:phrases[0],conflict:true});
simulate(true,{toast:phrases[0],noAsr:true});
console.log('V66_GATE_OFFLINE_PASS: 8 refusals x 2, 3 normals x 2, conflict, missing ASR; original OEM input untouched');