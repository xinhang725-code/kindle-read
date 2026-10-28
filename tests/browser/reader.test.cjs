const {test}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('@playwright/test');
const {spawn,execFileSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const binary=path.join(os.tmpdir(),'kindle-read-browser-test');
execFileSync('sh',['scripts/go.sh','build','-o',binary,'./cmd/kindle-read'],{stdio:'inherit'});
async function launch(data,source='qidian'){
 const p=spawn(binary,['serve','--source',source,'--data',data],{stdio:['ignore','pipe','pipe']});
 const url=await new Promise((resolve,reject)=>{let out='';const timer=setTimeout(()=>reject(new Error('startup timeout')),10000);p.stdout.on('data',b=>{out+=b;if(out.includes('\n')){clearTimeout(timer);resolve(out.trim());}});p.once('exit',code=>{clearTimeout(timer);reject(new Error('server exited '+code));});});
 return {p,url,async stop(){p.kill('SIGTERM');await new Promise(r=>p.once('exit',r));}};
}
test('desktop browser: import, read, paginate, change font and reopen at saved anchor',async(t)=>{
 const data=fs.mkdtempSync(path.join(os.tmpdir(),'kindle-read-ui-'));
 const server=await launch(data),browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_PATH || (fs.existsSync('/usr/bin/chromium')?'/usr/bin/chromium':undefined)});
 t.after(async()=>{await browser.close();await server.stop();fs.rmSync(data,{recursive:true,force:true});});
 const page=await browser.newPage({viewport:{width:824,height:1236}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(server.url);await page.getByRole('button',{name:'导入本地文本',exact:true}).click();
 await page.locator('#local-title').fill('阅读验证 · 合成文本');
 await page.locator('#local-text').fill(Array.from({length:60},(_,i)=>'第'+i+'段。春天的河岸很安静，晚风翻过书页。这里是一段用于检验分页的合成文字😀。'.repeat(3)).join('\n'));
 await page.getByRole('button',{name:'导入',exact:true}).click();
 await page.locator('.book').click();await page.locator('.chapter').click();
 await page.locator('#page-number').getByText(/^1 \/ /).waitFor();
 await page.waitForFunction(()=>document.getElementById('status').classList.contains('hidden'));
 await page.getByRole('button',{name:'下一页',exact:true}).click();
 await page.waitForFunction(()=>document.getElementById('page-number').textContent.indexOf('2 /')===0);
 await page.waitForTimeout(150);
 const before=await page.locator('#page').textContent();
 const beforeLayout=await page.evaluate(async()=>{const r=await fetch('api/state',{headers:{'X-Kindle-Read':'1'}});return (await r.json()).library.last;});
 await page.getByRole('button',{name:'增大字号'}).click();await page.waitForTimeout(150);
 const state=await page.evaluate(async()=>{const r=await fetch('api/state',{headers:{'X-Kindle-Read':'1'}});return r.json();});
 assert.equal(state.library.last.font_size,30);assert.ok(state.library.last.offset>0||state.library.last.paragraph>0);
 assert.deepEqual({p:state.library.last.paragraph,o:state.library.last.offset},{p:beforeLayout.paragraph,o:beforeLayout.offset},'typesetting must preserve the logical reading anchor');
 for(let i=0;i<3;i++){
  for(const name of ['减小字号','增大字号']){
   await page.getByRole('button',{name}).click();await page.waitForFunction(()=>document.getElementById('status').classList.contains('hidden'));
   const after=await page.evaluate(async()=>{const r=await fetch('api/state',{headers:{'X-Kindle-Read':'1'}});return (await r.json()).library.last;});
   assert.deepEqual({p:after.paragraph,o:after.offset},{p:beforeLayout.paragraph,o:beforeLayout.offset},'repeated font toggles drifted');
  }
 }
 const rendered=await page.locator('#page').textContent();assert.ok(rendered.length>0&&before.length>0);
 await page.reload();await page.locator('#reader:not(.hidden)').waitFor();
 assert.equal(await page.locator('#page').textContent(),rendered);
 assert.equal(await page.locator('#page').evaluate(n=>n.scrollHeight<=n.clientHeight),true, JSON.stringify(await page.evaluate(()=>['page','measure','catalog-open','chapter-heading'].map(id=>{const n=document.getElementById(id),s=getComputedStyle(n),b=n.getBoundingClientRect();return {id,scroll:n.scrollHeight,client:n.clientHeight,w:b.width,h:b.height,y:b.y,font:s.font,line:s.lineHeight};}))));
 for(const id of ['previous','font-down','font-up','next']){
  const box=await page.locator('#'+id).boundingBox();
  assert.ok(box.y+box.height<=1236, id+' must stay on screen');
 }
 assert.equal(await page.locator('#catalog-open').count(),1,'reader must expose a directory action after resume');
 fs.mkdirSync('test-results',{recursive:true});await page.screenshot({path:'test-results/reader.png'});
 await page.getByRole('button',{name:'书架',exact:true}).click();await page.locator('#shelf:not(.hidden)').waitFor();
 await page.screenshot({path:'test-results/shelf.png'});
 assert.deepEqual(errors,[]);
});
test('all three routes identify their own app; MiuRead does not claim content readiness',async(t)=>{
 const data=fs.mkdtempSync(path.join(os.tmpdir(),'kindle-read-three-'));const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_PATH || (fs.existsSync('/usr/bin/chromium')?'/usr/bin/chromium':undefined)});
 t.after(async()=>{await browser.close();fs.rmSync(data,{recursive:true,force:true});});
 for(const [source,title] of [['qidian','起点阅读'],['changpei','长佩阅读'],['miuread','MiuRead 独立版']]){
  const server=await launch(data,source);try{const page=await browser.newPage();await page.goto(server.url);await page.getByRole('heading',{name:title,exact:true}).waitFor();if(source==='miuread'){assert.ok(await page.getByRole('button',{name:'扫码登录微信读书'}).isVisible());assert.match(await page.locator('#intro').textContent(),/正文阅读仍在开发/);}await page.close();}finally{await server.stop();}
 }
});

test('a cancelled QR response cannot stop a newer login attempt',async(t)=>{
 const data=fs.mkdtempSync(path.join(os.tmpdir(),'kindle-read-login-ui-')),server=await launch(data,'miuread');
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_PATH || (fs.existsSync('/usr/bin/chromium')?'/usr/bin/chromium':undefined)});
 t.after(async()=>{await browser.close();await server.stop();fs.rmSync(data,{recursive:true,force:true});});
 const page=await browser.newPage();let generation=0;const polls=[],waiters=[];
 const observed=n=>new Promise((resolve,reject)=>{if(polls.length>=n){resolve();return;}const timer=setTimeout(()=>reject(new Error('new login stopped polling')),5000);waiters.push(()=>{if(polls.length>=n){clearTimeout(timer);resolve();}});});
 await page.route('**/api/login/start',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({generation:++generation,url:'qr.png',state:'waiting'})}));
 await page.route('**/api/login/poll',route=>{polls.push(route);waiters.forEach(w=>w());});
 await page.goto(server.url);await page.getByRole('button',{name:'扫码登录微信读书'}).click();await observed(1);
 await page.getByRole('button',{name:'取消登录',exact:true}).click();await page.getByRole('button',{name:'扫码登录微信读书'}).click();await observed(2);
 await polls[0].fulfill({status:400,contentType:'application/json',body:'{"error":"OLD ATTEMPT FAILED"}'});
 await polls[1].fulfill({status:200,contentType:'application/json',body:'{"generation":2,"state":"waiting"}'});
 await observed(3);assert.ok(!(await page.locator('#status').textContent()).includes('OLD ATTEMPT FAILED'));
 await polls[2].fulfill({status:200,contentType:'application/json',body:'{"generation":2,"state":"waiting"}'});
});

test('a stale saved chapter falls back to a selectable directory',async(t)=>{
 const data=fs.mkdtempSync(path.join(os.tmpdir(),'kindle-read-stale-')),server=await launch(data);
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_PATH || (fs.existsSync('/usr/bin/chromium')?'/usr/bin/chromium':undefined)});
 t.after(async()=>{await browser.close();await server.stop();fs.rmSync(data,{recursive:true,force:true});});
 const page=await browser.newPage();let chapterRequests=0;
 const stale={book_id:'book',chapter_id:'removed',paragraph:0,offset:0,font_size:28};
 await page.route('**/api/state',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({source:'qidian',name:'起点阅读',account:{},library:{books:[{id:'book',title:'更新后的作品',chapters:[{id:'new',title:'当前章节'}]}],last:stale,progress:{book:stale}}})}));
 page.on('request',r=>{if(r.url().includes('/api/chapter?'))chapterRequests++;});
 await page.goto(server.url);await page.locator('.book').click();await page.getByRole('button',{name:'当前章节',exact:true}).waitFor();
 assert.equal(chapterRequests,0,'stale resume should not issue a missing chapter request');
});
