const {test}=require('node:test');
const assert=require('node:assert/strict');
const reader=require('../internal/web/assets/reader.js');
const measure=(s)=>Array.from(s).length<=4;
test('pagination preserves every character, paragraph and emoji',()=>{
 const paragraphs=['第一段你好😀继续','abc中文','末尾'];
 const pages=reader.paginate(paragraphs,measure);
 assert.ok(pages.length>3);
 for(let p=0;p<paragraphs.length;p++) {
  const text=pages.flatMap(x=>x.parts).filter(x=>x.paragraph===p).map(x=>x.text).join('');
  assert.equal(text,paragraphs[p]);
 }
 assert.ok(!pages.some(x=>x.parts.some(p=>/^[\uDC00-\uDFFF]|[\uD800-\uDBFF]$/.test(p.text))));
});
test('resume maps anchor after layout changes and clamps end of chapter',()=>{
 const pages=reader.paginate(['123456789','abc'],measure);
 assert.equal(reader.locate(pages,{paragraph:0,offset:5}),1);
 const wider=reader.paginate(['123456789','abc'],s=>Array.from(s).length<=8);
 assert.equal(reader.locate(wider,{paragraph:0,offset:5}),0);
 assert.equal(reader.locate(pages,{paragraph:99,offset:99}),pages.length-1);
});
test('empty or too-small viewport cannot loop forever',()=>{
 assert.deepEqual(reader.paginate([],measure),[]);
 assert.throws(()=>reader.paginate(['a'],()=>false),/viewport/);
 assert.throws(()=>reader.paginate(['a'],null));
});
test('Chinese page boundaries bind closing punctuation and opening brackets without dropping text',()=>{
 const paragraphs=['甲乙丙，丁戊（己庚）辛壬。癸子：丑寅？！卯辰'];
 const pages=reader.paginate(paragraphs,s=>Array.from(s).length<=5);
 assert.equal(pages.flatMap(p=>p.parts).map(p=>p.text).join(''),paragraphs[0]);
 for(const page of pages){const s=page.parts.map(p=>p.text).join('');assert.ok(!/^[。，、；：？！）》」』】…]/.test(s),s);assert.ok(!/[（《「『【]$/.test(s),s);}
});
