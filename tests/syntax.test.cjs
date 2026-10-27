const {test}=require('node:test');
const fs=require('node:fs');
const acorn=require('acorn');
test('device JavaScript parses as ES5 for Mesquite',()=>{
 for(const file of ['app.js','reader.js']) acorn.parse(fs.readFileSync('internal/web/assets/'+file,'utf8'),{ecmaVersion:5});
});
