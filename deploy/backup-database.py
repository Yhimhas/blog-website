"""Create retained PostgreSQL backups without credentials on the command line."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import urllib.parse

parser = argparse.ArgumentParser()
parser.add_argument('--env-file', type=Path, required=True)
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
settings = {}
for line in args.env_file.read_text().splitlines():
    if not line or line.startswith('#') or '=' not in line:
        continue
    key, value = line.split('=', 1)
    settings[key] = value.strip().strip("'").replace("\\'", "'")
url = urllib.parse.urlsplit(settings['DATABASE_URL'])
database = url.path.lstrip('/')
if not database or url.scheme not in ['postgres', 'postgresql']:
    raise SystemExit('A PostgreSQL database URL is required')
env = dict(os.environ, PGHOST=url.hostname or '', PGPORT=str(url.port or 5432),
           PGUSER=urllib.parse.unquote(url.username or ''),
           PGPASSWORD=urllib.parse.unquote(url.password or ''), PGDATABASE=database)
params = urllib.parse.parse_qs(url.query)
for key in ['options', 'sslmode']:
    if key in params:
        env['PG' + key.upper()] = params[key][0]
args.output.mkdir(parents=True, exist_ok=True)
args.output.chmod(0o700)
stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + secrets.token_hex(3)
partial = args.output / (stamp + '.partial')
dump = args.output / (stamp + '.dump')
os.umask(0o077)
subprocess.run(['pg_dump', '-Fc', '--file', str(partial)], env=env, check=True)
subprocess.run(['pg_restore', '--list', str(partial)], stdout=subprocess.DEVNULL, check=True)
partial.rename(dump)
metadata = {'database': database, 'file': dump.name, 'bytes': dump.stat().st_size,
            'sha256': hashlib.sha256(dump.read_bytes()).hexdigest()}
(args.output / (stamp + '.json')).write_text(json.dumps(metadata, indent=2))
print(json.dumps(metadata))
