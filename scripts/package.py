"""Create a development USB package and its exact corresponding source. No user data."""
from pathlib import Path
import hashlib
import io
import os
import subprocess
import sys
import tarfile
import zipfile

root = Path(__file__).resolve().parent.parent
stage = Path(sys.argv[1])
go_root = Path(sys.argv[2])
dist = root / 'dist'
source = dist / 'kindle-read-source.tar.gz'
entries = ['cmd', 'internal', 'tests', 'scripts', 'docs', 'third_party', '.superpowers', '.gitignore',
           'README.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'go.mod', 'go.sum',
           'package.json', 'package-lock.json']
with tarfile.open(source, 'w:gz') as archive:
    for entry in entries:
        archive.add(root / entry, arcname='kindle-read/' + entry)
    archive.add(stage / 'vendor', arcname='kindle-read/vendor')

target = dist / 'kindle-read-0.1.0-alpha.1-armv7.zip'
def put(z, name, data, executable=False):
    info = zipfile.ZipInfo(name, date_time=(2026, 10, 27, 0, 0, 0))
    info.external_attr = (0o100755 if executable else 0o100644) << 16
    info.compress_type = zipfile.ZIP_DEFLATED
    z.writestr(info, data)

with zipfile.ZipFile(target, 'w') as archive:
    base = 'extensions/kindle-read/'
    put(archive, base + 'bin/kindle-read', (dist / 'kindle-read-armv7').read_bytes(), True)
    put(archive, base + 'source.tar.gz', source.read_bytes())
    for name in ['LICENSE', 'THIRD_PARTY_NOTICES.md']:
        put(archive, base + name, (root / name).read_bytes())
    put(archive, base + 'INSTALL.md', (root / 'docs/installation.md').read_bytes())
    put(archive, base + 'third_party/Go.LICENSE', (go_root / 'LICENSE').read_bytes())
    for notice in sorted((root / 'third_party').iterdir()):
        put(archive, base + 'third_party/' + notice.name, notice.read_bytes())
    for name, path in [('x-net', 'golang.org/x/net/LICENSE'), ('go-qrcode', 'github.com/skip2/go-qrcode/LICENSE')]:
        put(archive, base + 'third_party/' + name + '.LICENSE', (stage / 'vendor' / path).read_bytes())
    for name, title, command in [('Setup', '安装 Kindle Read 测试版', 'install'), ('Uninstall', '卸载 Kindle Read 入口', 'uninstall')]:
        script = '#!/bin/sh\n# Name: ' + title + '\n# Author: kindle-read\nexec /mnt/us/extensions/kindle-read/bin/kindle-read ' + command + '\n'
        put(archive, 'documents/KindleRead-' + name + '.sh', script.encode(), True)

files = [target, source, dist / 'kindle-read-armv7', dist / 'kindle-read-desktop']
(dist / 'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest() + '  ' + p.name + '\n' for p in files))
print('Built development package:', target.name)
print('Includes source and dependency notices. Kindle and live provider acceptance remain pending.')
