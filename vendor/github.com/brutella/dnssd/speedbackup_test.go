package dnssd
import("net";"testing";"github.com/miekg/dns")
func TestSpeedBackupIPDedup(t *testing.T) {
 s:=newService("test._webdav._tcp.local.")
 a:=&net.Interface{Name:"first"}; b:=&net.Interface{Name:"second"}
 for i:=0;i<10000;i++ { s.addIP(net.ParseIP("192.0.2.1"),a);s.addIP(net.IP{192,0,2,1},b) }
 if len(s.IPs)!=1 || len(s.ifaceIPs[a.Name])!=1 || len(s.ifaceIPs[b.Name])!=1 {t.Fatal("duplicate addresses accumulated")}
}
func TestSpeedBackupSameHostDifferentPort(t *testing.T) {
 s,err:=NewService(Config{Name:"test",Type:"_webdav._tcp",Host:"same",Port:8765});if err!=nil {t.Fatal(err)}
 for _,port:=range []uint16{8765,9} {
  rr:=SRV(s);rr.Port=port
  records:=filterRecords(&Request{msg:&dns.Msg{Answer:[]dns.RR{rr}}},&s)
  if (len(records)>0)!=(port!=8765) {t.Fatalf("port %d incorrectly classified",port)}
 }
}
