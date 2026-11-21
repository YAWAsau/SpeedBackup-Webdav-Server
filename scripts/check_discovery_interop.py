"""Independent python-zeroconf client against the production Go adapter.
Install zeroconf==0.154.0; build sbserver tests to dist/discovery-interop.exe.
All fixtures use ephemeral ports and never touch the installed server.
"""
import ipaddress,json,os,queue,socket,subprocess,tempfile,time,threading
from pathlib import Path
from zeroconf import Zeroconf,ServiceBrowser,ServiceInfo,IPVersion,ServiceStateChange

root=Path(__file__).resolve().parents[1]
out=root/'dist/discovery-interop';out.mkdir(parents=True,exist_ok=True)
kind='_webdav._tcp.local.'
results=[]
for family,conflict in [(IPVersion.V4Only,False),(IPVersion.V6Only,False),(IPVersion.All,True)]:
 events=queue.Queue()
 def changed(zeroconf,service_type,name,state_change):events.put((name,state_change))
 zc=Zeroconf(ip_version=family);browser=ServiceBrowser(zc,kind,handlers=[changed])
 fake=None;child=None;advertising=threading.Event();announcer=None
 with tempfile.TemporaryDirectory(dir=out) as temp:
  ready=Path(temp)/'ready.json';stop=Path(temp)/'stop'
  try:
   if conflict:
    ip=socket.gethostbyname(socket.gethostname())
    fake=ServiceInfo(kind,'SpeedBackup-Interop.'+kind,addresses=[socket.inet_aton(ip)],port=9,properties={'path':'/'},server='speedbackup-interop.local.')
    zc.register_service(fake)
    def defend():
     while not advertising.wait(.5):zc.update_service(fake)
    announcer=threading.Thread(target=defend);announcer.start()
   env={**os.environ,'SB_INTEROP_READY':str(ready),'SB_INTEROP_STOP':str(stop),'SB_INTEROP_CONFLICT':'1' if conflict else '0'}
   child=subprocess.Popen([str(root/'dist/discovery-interop.exe'),'-test.run=^TestDiscoveryInteropHarness$','-test.v'],env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
   until=time.monotonic()+10
   while not ready.exists() and time.monotonic()<until:time.sleep(.05)
   assert ready.exists(),'fixture did not start'
   port=json.loads(ready.read_text())['port'];found=None;until=time.monotonic()+15
   while time.monotonic()<until:
    try:name,state=events.get(timeout=.3)
    except queue.Empty:continue
    if state==ServiceStateChange.Removed:continue
    info=zc.get_service_info(kind,name,timeout=1500)
    if info and info.port==port:found=info;break
   assert found is not None,f'{family}: independent resolver did not find service'
   assert found.properties[b'path']==b'/'
   assert found.properties[b'id']==b'1234567890abcdef1234567890abcdef'
   assert set(found.properties)=={b'path',b'txtvers',b'product',b'id'}
   ips=found.parsed_addresses();assert ips and not any(ipaddress.ip_address(ip).is_loopback for ip in ips)
   if conflict:assert found.name!='SpeedBackup-Interop.'+kind,'conflicting name was not renamed'
   stop.touch();child.wait(timeout=8);assert child.returncode==0,child.stdout.read().decode(errors='replace')
   removed=False;until=time.monotonic()+6
   while time.monotonic()<until:
    try:name,state=events.get(timeout=.3)
    except queue.Empty:continue
    if name==found.name and state==ServiceStateChange.Removed:removed=True;break
   assert removed,'goodbye was not observed by independent client'
   result={'transport':family.name,'conflict':conflict,'name':found.name,'port':port,'addresses':ips,'txt':{k.decode():v.decode() for k,v in found.properties.items()},'goodbye':removed}
   results.append(result);print(json.dumps(result),flush=True)
  finally:
   stop.touch()
   if child and child.poll() is None:
    try:child.wait(timeout=8)
    except subprocess.TimeoutExpired:child.kill();child.wait()
   advertising.set()
   if announcer:announcer.join()
   if fake:zc.unregister_service(fake)
   browser.cancel();zc.close()
(out/'results.json').write_text(json.dumps(results,indent=2))
print('PASS: independent IPv4/IPv6 DNS-SD resolution, SRV/TXT/addresses, coexistence, name conflict and goodbye')
