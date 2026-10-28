"""Create a development USB package and its exact corresponding source. No user data."""
from pathlib import Path
import hashlib
import gzip
import io
import json
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
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
epoch = int(subprocess.check_output(['git', 'log', '-1', '--format=%ct'], cwd=root, text=True).strip())
build_info = json.dumps({'version': '0.2.0-alpha.1', 'revision': revision,
                        'dirty': bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=root)),
                        'go': '1.25.3', 'target': 'linux/arm GOARM=7 CGO_ENABLED=0'}, indent=2).encode()
entries = ['cmd', 'internal', 'tests', 'scripts', 'docs', 'third_party', '.superpowers', '.gitignore',
           'README.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'go.mod', 'go.sum',
           'package.json', 'package-lock.json']
def normalize(info):
    info.uid = info.gid = 0
    info.uname = info.gname = ''
    info.mtime = epoch
    info.mode = 0o755 if info.isdir() or info.mode & 0o111 else 0o644
    return info

with source.open('wb') as output, gzip.GzipFile(filename='', mode='wb', fileobj=output, mtime=epoch) as compressed:
    with tarfile.open(fileobj=compressed, mode='w') as archive:
        for entry in entries:
            archive.add(root / entry, arcname='kindle-read/' + entry, filter=normalize)
        archive.add(stage / 'vendor', arcname='kindle-read/vendor', filter=normalize)
        info = normalize(tarfile.TarInfo('kindle-read/BUILD.json'))
        info.size = len(build_info)
        archive.addfile(info, io.BytesIO(build_info))

target = dist / 'kindle-read-0.2.0-alpha.1-armv7.zip'
changpei_target = dist / 'changpei-0.2.0-alpha.1-armv7.zip'
def put(z, name, data, executable=False):
    info = zipfile.ZipInfo(name, date_time=(2026, 10, 27, 0, 0, 0))
    info.external_attr = (0o100755 if executable else 0o100644) << 16
    info.compress_type = zipfile.ZIP_DEFLATED
    z.writestr(info, data)

for package_target, only_changpei in [(target, False), (changpei_target, True)]:
    with zipfile.ZipFile(package_target, 'w') as archive:
        base = 'extensions/kindle-read/'
        package_info = json.loads(build_info)
        package_info['install_sources'] = ['changpei'] if only_changpei else ['qidian', 'changpei', 'miuread']
        put(archive, base + 'BUILD.json', json.dumps(package_info, indent=2).encode())
        put(archive, base + 'bin/kindle-read', (dist / 'kindle-read-armv7').read_bytes(), True)
        put(archive, base + 'source.tar.gz', source.read_bytes())
        for name in ['LICENSE', 'THIRD_PARTY_NOTICES.md']:
            put(archive, base + name, (root / name).read_bytes())
        put(archive, base + 'INSTALL.md', (root / ('docs/changpei-installation.md' if only_changpei else 'docs/installation.md')).read_bytes())
        put(archive, base + 'third_party/Go.LICENSE', (go_root / 'LICENSE').read_bytes())
        for notice in sorted((root / 'third_party').iterdir()):
            put(archive, base + 'third_party/' + notice.name, notice.read_bytes())
        for name, path in [('x-net', 'golang.org/x/net/LICENSE'), ('go-qrcode', 'github.com/skip2/go-qrcode/LICENSE')]:
            put(archive, base + 'third_party/' + name + '.LICENSE', (stage / 'vendor' / path).read_bytes())
        for name, title, command in [('Setup', '安装 Kindle Read 测试版', 'install'), ('Uninstall', '卸载 Kindle Read 入口', 'uninstall')]:
            if only_changpei:
                title = '安装长佩阅读测试版' if command == 'install' else '卸载长佩阅读入口'
                command += ' changpei'
                name = 'Changpei-' + name
            script = '#!/bin/sh\n# Name: ' + title + '\n# Author: kindle-read\nexec /mnt/us/extensions/kindle-read/bin/kindle-read ' + command + '\n'
            put(archive, 'documents/KindleRead-' + name + '.sh', script.encode(), True)
files = [target, changpei_target, source, dist / 'kindle-read-armv7', dist / 'kindle-read-desktop']
(dist / 'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest() + '  ' + p.name + '\n' for p in files))
print('Built development packages:', target.name, changpei_target.name)
print('Includes source and dependency notices. Kindle and live provider acceptance remain pending.')
