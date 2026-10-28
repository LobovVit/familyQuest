import pathlib,subprocess,datetime,os,secrets,time,shutil
os.umask(0o077)
base=pathlib.Path('/home/lobov/lovit-identity')
dest=pathlib.Path('/home/lobov/familyquest-backups')/('identity-restore-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ'));dest.mkdir(parents=True)
def run(args,**kw):return subprocess.run(args,check=True,**kw)
with (dest/'identity.dump').open('wb') as out:run(['docker','exec','lovit-identity-postgres-1','pg_dump','-U','postgres','-d','zitadel','-Fc'],stdout=out)
for f in ['.env','clients.json','imported-users.json']:shutil.copy2(base/f,dest/f)
run(['docker','cp','lovit-identity-identity-1:/bootstrap',str(dest/'bootstrap')],stdout=subprocess.DEVNULL)
name='lovit-restore-check-'+secrets.token_hex(5)
identity_name=name+'-identity'
try:
 run(['docker','run','-d','--name',name,'--network','none','--tmpfs','/var/lib/postgresql/data','-e','POSTGRES_HOST_AUTH_METHOD=trust','postgres:17-alpine'],stdout=subprocess.DEVNULL)
 for _ in range(40):
  if subprocess.run(['docker','exec',name,'pg_isready','-U','postgres'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0:break
  time.sleep(1)
 run(['docker','exec',name,'createdb','-U','postgres','zitadel'])
 with (dest/'identity.dump').open('rb') as src:run(['docker','exec','-i',name,'pg_restore','-U','postgres','-d','zitadel','--no-owner','--no-acl','--exit-on-error'],stdin=src,stdout=subprocess.DEVNULL)
 query="select count(*) from eventstore.events2"
 def count(c):return run(['docker','exec',c,'psql','-U','postgres','-d','zitadel','-Atc',query],capture_output=True,text=True).stdout.strip()
 restored=count(name)
 run(['docker','exec',name,'psql','-U','postgres','-d','zitadel','-Atc','select 1 from projections.users14 limit 1'],capture_output=True,text=True)
 # Start the restored identity with the saved master key in the isolated DB network namespace.
 config=dict(line.split('=',1) for line in (dest/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
 envfile=dest/'restore-runtime.env'
 envfile.write_text('ZITADEL_MASTERKEY='+config['IDENTITY_MASTERKEY']+'\nZITADEL_DATABASE_POSTGRES_DSN=postgres://postgres@127.0.0.1:5432/zitadel?sslmode=disable\nZITADEL_EXTERNALDOMAIN=auth.lovit.tech\nZITADEL_EXTERNALPORT=443\nZITADEL_EXTERNALSECURE=true\nZITADEL_TLS_ENABLED=false\nZITADEL_PORT=8080\nZITADEL_MACHINE_IDENTIFICATION_PRIVATEIP_ENABLED=false\nZITADEL_MACHINE_IDENTIFICATION_HOSTNAME_ENABLED=true\nZITADEL_MACHINE_IDENTIFICATION_WEBHOOK_ENABLED=false\n')
 run(['docker','run','-d','--name',identity_name,'--network','container:'+name,'--env-file',str(envfile),'ghcr.io/zitadel/zitadel:v4.16.0','start','--masterkeyFromEnv'],stdout=subprocess.DEVNULL)
 healthy=False
 for _ in range(40):
  if subprocess.run(['docker','exec',identity_name,'/app/zitadel','ready'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0:
   healthy=True;break
  time.sleep(1)
 if not healthy:
  with (dest/'restore-startup.log').open('wb') as log:subprocess.run(['docker','logs',identity_name],stdout=log,stderr=log)
  raise RuntimeError('Restored identity did not become ready; protected log saved in backup directory')
 envfile.unlink()
 (dest/'VERIFIED').write_text('PostgreSQL 17 restore and isolated ZITADEL startup with saved master key passed; eventstore rows='+restored+'\n')
 print('Restore verified:',dest,'eventstore rows:',restored)
finally:
 subprocess.run(['docker','rm','-f',identity_name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 subprocess.run(['docker','rm','-f',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
