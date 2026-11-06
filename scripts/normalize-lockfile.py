#!/usr/bin/env python3
"""Point every "resolved" URL in web/package-lock.json at registry.npmjs.org.

npm records the tarball URL of whatever registry it installed from, so an
install through a local mirror (e.g. mirrors.tencentyun.com/npm/...) leaves
URLs that CI and other users cannot reach. The tarballs are identical, so the
integrity hashes stay valid. npm still installs through a configured mirror,
because it maps registry.npmjs.org URLs onto the configured registry.

    scripts/normalize-lockfile.py [path/to/package-lock.json]
"""
import re
import sys

PUBLIC = "https://registry.npmjs.org/"
# <any mirror prefix>/<@scope/name or name>/-/<file>.tgz
MIRROR = re.compile(r'"resolved": "https?://(?!registry\.npmjs\.org/)[^"]*?/((?:@[^/"]+/)?[^/"@]+)/-/')

path = sys.argv[1] if len(sys.argv) > 1 else "web/package-lock.json"
with open(path) as f:
    text = f.read()
text, count = MIRROR.subn(lambda m: f'"resolved": "{PUBLIC}{m.group(1)}/-/', text)
with open(path, "w") as f:
    f.write(text)
print(f"{path}: {count} URL(s) pointed at {PUBLIC}")
