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
 last={book_id:'42',chapter_id:'101',paragraph:blocks.length-1,offset:0,font_size:22};
 await page.reload();await page.locator('#page .author-note').waitFor();assert.match(await page.locator('#page .author-note').textContent(),/测试附言/);
 assert.equal(await page.locator('#page .author-note').evaluate(n=>getComputedStyle(n).borderLeftWidth),'3px');
});

test('page-turn authorization expiry clears rendered text and returns to login',async(t)=>{
 const {page,url}=await setup(t);let saves=0;
 const book={id:'42',title:'会话测试',chapters:[{id:'101',title:'第一章'}]};
 await page.route('**/api/state',r=>json(r,{source:'changpei',name:'长佩阅读',account:{id:'1',name:'测试'},login_required:false,library:{books:[book],progress:{}}}));
 await page.route('**/api/chapter?*',r=>json(r,{book,chapter:book.chapters[0],paragraphs:Array.from({length:15},()=>('一段合成正文，用来检查会话失效。').repeat(20))}));
 await page.route('**/api/progress',r=>++saves===1?json(r,{saved:true}):json(r,{code:'AUTH_EXPIRED',error:'登录过期',message:'登录过期'},401));
 await page.goto(url);await page.locator('.book').click();await page.locator('.chapter').click();await page.waitForFunction(()=>document.querySelector('#page').textContent.length>0);await page.waitForFunction(()=>document.getElementById('status').classList.contains('hidden'));
 await page.locator('#page').click({position:{x:700,y:120}});await page.locator('#login:not(.hidden)').waitFor();
 assert.equal(await page.locator('#reader').isVisible(),false);assert.equal(await page.locator('#page').textContent(),'');assert.equal(await page.locator('#back').isEnabled(),false);
});

test('search splits an upstream result batch into pages that fit the screen',async(t)=>{
 const {page,url}=await setup(t);let queries=0;
 const books=Array.from({length:10},(_,i)=>({id:String(i+1),title:'合成的长标题'+('用于验证分页和版面。').repeat(8)+i,author:'合成作者'}));
 await page.route('**/api/state',r=>json(r,{source:'changpei',name:'长佩阅读',account:{id:'1',name:'测试'},login_required:false,library:{books:[],progress:{}}}));
 await page.route('**/api/search',r=>{queries++;return json(r,{books,count:20,page:1})});
 await page.goto(url);await page.getByRole('button',{name:'搜索作品',exact:true}).click();await page.locator('#search-query').fill('合成');await page.getByRole('button',{name:'搜索',exact:true}).click();await page.locator('#results .book').first().waitFor();
 assert.ok(await page.locator('#results .book').count()<10);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight),true);
 const before=await page.locator('#results .book').first().textContent();await page.locator('#search-pages').getByRole('button',{name:'下一页',exact:true}).click();
 assert.notEqual(await page.locator('#results .book').first().textContent(),before);assert.equal(queries,1,'in-batch navigation should not contact upstream');
 assert.equal(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight),true);
 fs.mkdirSync('test-results',{recursive:true});await page.screenshot({path:'test-results/changpei-search.png'});
});

test('search pagination reaches the next upstream batch after a short screen page',async(t)=>{
 const {page,url}=await setup(t);await page.setViewportSize({width:824,height:1000});
 const all=Array.from({length:21},(_,i)=>({id:String(i+1),title:'作品 '+(i+1),author:'合成作者'}));
 await page.route('**/api/state',r=>json(r,{source:'changpei',name:'长佩阅读',account:{id:'1',name:'测试'},login_required:false,library:{books:[],progress:{}}}));
 await page.route('**/api/search',r=>{const p=r.request().postDataJSON().page;return json(r,{books:all.slice((p-1)*10,p*10),count:21,page:p})});
 await page.goto(url);await page.getByRole('button',{name:'搜索作品',exact:true}).click();await page.locator('#search-query').fill('作品');await page.getByRole('button',{name:'搜索',exact:true}).click();await page.locator('#results .book').first().waitFor();
 const seen=[];
 for(let i=0;i<24;i++){
  seen.push(...await page.locator('#results .book-title').allTextContents());
  const next=page.locator('#search-pages').getByRole('button',{name:'下一页',exact:true});if(!(await next.isEnabled()))break;
  await next.click();await page.waitForFunction(()=>document.getElementById('status').classList.contains('hidden'));
 }
 assert.deepEqual(seen,all.map(b=>b.title));
});
