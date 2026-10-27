"""Static inventory of the pinned upstream; does not execute its Lua or native libraries."""
from pathlib import Path
from collections import Counter
import json
import re
import subprocess
import sys

root = Path(sys.argv[1]).resolve()
pin = '581f9b660097aa42fe33170098346862cf0a9bbf'
actual = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
if actual != pin:
    raise SystemExit('Upstream SHA does not match the migration baseline')
modules = []
groups = Counter()
for path in sorted((root / 'miuread.koplugin').rglob('*.lua')):
    code = path.read_text()
    dependencies = sorted(set(re.findall(r'''require\s*\(?\s*["']([^"']+)''', code)))
    for name in dependencies:
        groups[name.split('/')[0].split('.')[0]] += 1
    modules.append({'path': str(path.relative_to(root)), 'lines': len(code.splitlines()), 'static_requires': dependencies})
report = {'upstream': 'https://github.com/miumiupy98-art/miuread-koreader', 'commit': pin,
          'scope': 'Static literal require inventory only. Dynamic imports, globals, native ABI and runtime reachability are not certified.',
          'lua_files': len(modules), 'lua_lines': sum(m['lines'] for m in modules),
          'require_groups_per_module': dict(groups.most_common()), 'modules': modules}
json.dump(report, sys.stdout, ensure_ascii=False, indent=2)
print()
