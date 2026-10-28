const {test}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('@playwright/test');
const {spawn,execFileSync}=require('node:child_process');
const fs=require('node:fs'),os=require('node:os'),path=require('node:path');
const binary=path.join(os.tmpdir(),'changpei-browser-test');
execFileSync('sh',['scripts/go.sh','build','-o',binary,'./cmd/kindle-read']);
async function setup(t){
 const data=fs.mkdtempSync(path.join(os.tmpdir(),'changpei-ui-'));
 const p=spawn(binary,['serve','--source','changpei','--data',data]);
 const url=await new Promise((resolve,reject)=>{let s='';p.stdout.on('data',b=>{s+=b;if(s.includes('\n'))resolve(s.trim())});p.on('error',reject)});
 const browser=await chromium.launch({headless:true,executablePath:'/usr/bin/chromium'});
 t.after(async()=>{await browser.close();p.kill();await new Promise(r=>p.once('exit',r));fs.rmSync(data,{recursive:true,force:true})});
 const page=await browser.newPage({viewport:{width:824,height:1236}});page.setDefaultTimeout(6000);return {page,url};
}
const json=(route,value,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(value)});
test('first launch is QR-only, scanned waits for confirmation, expired session closes reader',async(t)=>{
 const {page,url}=await setup(t);let logged=false,poll=0;
 await page.route('**/api/state',r=>json(r,{source:'changpei',name:'长佩阅读',account:logged?{id:'1',name:'测试读者'}:{},login_required:!logged,library:{books:[],progress:{}}}));
 await page.route('**/api/login/start',r=>json(r,{generation:1,state:'waiting',url:'qr.png'}));
 await page.route('**/api/login/poll',r=>{poll++;if(poll===1)return json(r,{generation:1,state:'scanned'});logged=true;return json(r,{generation:1,state:'ready'})});
 await page.goto(url);await page.locator('#login:not(.hidden)').waitFor();
 assert.equal(await page.locator('#shelf').isVisible(),false);assert.equal(await page.locator('#local-open').isVisible(),false);
 await page.getByText('扫码成功，请在手机上确认。',{exact:true}).waitFor();
 assert.equal(await page.locator('#shelf').isVisible(),false);
 await page.locator('#shelf:not(.hidden)').waitFor();assert.match(await page.locator('#intro').textContent(),/测试读者/);
 await page.route('**/api/search',r=>json(r,{code:'AUTH_EXPIRED',error:'登录已过期',message:'登录已过期'},401));
 await page.getByRole('button',{name:'搜索作品',exact:true}).click();await page.locator('#search-query').fill('测试');
 await page.getByRole('button',{name:'搜索',exact:true}).click();await page.locator('#login:not(.hidden)').waitFor();
 assert.equal(await page.locator('#shelf').isVisible(),false);
});
test('Changpei paged reader hides controls, preserves anchor and renders author notes',async(t)=>{
 const {page,url}=await setup(t);let last=null;const book={id:'42',title:'排版验证',author:'合成作者',chapters:[{id:'101',title:'第一章'}]};
 const blocks=Array.from({length:18},(_,i)=>({t:'p',text:'第'+i+'段。'+('这是一段用于验证中文分页的文字（以及括号）。😀这是合成文本，测试用。').repeat(4)}));blocks.push({t:'note',text:'作者的话：测试附言。'});
 await page.route('**/api/state',r=>json(r,{source:'changpei',name:'长佩阅读',account:{id:'1',name:'测试'},login_required:false,library:{books:[book],progress:last?{'42':last}:{},last}}));
 await page.route('**/api/chapter?*',r=>json(r,{book,chapter:book.chapters[0],blocks,paragraphs:blocks.map(b=>b.text)}));
 await page.route('**/api/progress',r=>{last=r.request().postDataJSON();return json(r,{saved:true})});
 await page.goto(url);await page.locator('.book').click();await page.locator('.chapter').click();await page.locator('#reader:not(.hidden)').waitFor();await page.waitForFunction(()=>document.querySelector('#page').textContent.length>0);
 assert.equal(await page.locator('#reader-nav').isVisible(),false);
 assert.equal(await page.locator('#page').evaluate(n=>getComputedStyle(n).fontSize),'20px');
 await page.locator('#page').click({position:{x:700,y:120}});await page.waitForFunction(()=>document.getElementById('reading-position').textContent.indexOf('2 /')>=0);
 const before={para:last.paragraph,offset:last.offset};
 await page.locator('#page').click({position:{x:380,y:120}});await page.locator('#font-up').click();await page.waitForFunction(()=>document.getElementById('status').classList.contains('hidden'));
 assert.deepEqual({para:last.paragraph,offset:last.offset},before);assert.equal(last.font_size,22);
 const text=await page.locator('#page').textContent();await page.reload();await page.locator('#reader:not(.hidden)').waitFor();assert.equal(await page.locator('#page').textContent(),text);
 assert.equal(await page.locator('#page').evaluate(n=>n.scrollHeight<=n.clientHeight),true);
 fs.mkdirSync('test-results',{recursive:true});await page.screenshot({path:'test-results/changpei-reader.png'});
 // Body stays within the viewport, and opening controls doesn't reflow its text.
 const beforeOpen=await page.locator('#page').textContent();await page.locator('#page').click({position:{x:380,y:120}});assert.equal(await page.locator('#page').textContent(),beforeOpen);
 for(const id of ['font-up','font-down','next','catalog-open']){const box=await page.locator('#'+id).boundingBox();assert.ok(box.width>=48&&box.height>=48,id)}
 await page.screenshot({path:'test-results/changpei-controls.png'});
});
