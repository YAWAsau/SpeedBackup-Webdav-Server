// Test driver for our own native window and a loopback mock server only.
using System;
using System.IO;
using System.Reflection;
using System.Threading.Tasks;
using System.Windows.Forms;
using SpeedBackupDesktop;
class PowerPromptSmoke {
 [STAThread]static int Main(string[] args){int result=1;Application.EnableVisualStyles();using(var form=new Prompt(args[0],false)){
  form.StartPosition=FormStartPosition.Manual;form.Location=new System.Drawing.Point(-32000,-32000);form.ShowInTaskbar=false;
  var flags=BindingFlags.Instance|BindingFlags.NonPublic;var send=typeof(Prompt).GetMethod("Send",flags);var data=typeof(Prompt).GetField("data",flags);
  form.Shown+=async(s,e)=>{try{
   await (Task)send.Invoke(form,new object[]{"heartbeat",true});if(!form.TopMost||form.WindowState==FormWindowState.Minimized)throw new Exception("Prompt is not configured visible/topmost");var state=(Snapshot)data.GetValue(form);if(args.Length>2&&args[2]=="fault"){File.WriteAllText(args[1],"PASS: UI exception pauses server and automatically closes");result=0;typeof(Prompt).GetMethod("FailSafe",flags).Invoke(form,new object[]{new InvalidOperationException("controlled UI test")});return;}if(args.Length>2){if(state==null||state.device==null||state.device.id.Length!=64)throw new Exception("Missing device fingerprint");await (Task)send.Invoke(form,new object[]{"approve",true});if(((Snapshot)data.GetValue(form)).job.state!="approved")throw new Exception("Device approval failed");File.WriteAllText(args[1],"PASS: native pairing fingerprint / approval transport");result=0;return;}if(state==null||!state.job.preview||state.job.outcome!="failed"||state.job.operation!="restore")throw new Exception("Native JSON/heartbeat decode failed");
   if((bool)typeof(Prompt).GetField("dark",flags).GetValue(form)||(bool)typeof(Prompt).GetField("glass",flags).GetValue(form)!=true)throw new Exception("Initial white glass theme not applied");
   await (Task)send.Invoke(form,new object[]{"defer",true});if(!(bool)typeof(Prompt).GetField("dark",flags).GetValue(form)||(bool)typeof(Prompt).GetField("glass",flags).GetValue(form))throw new Exception("Live theme update not applied");if(((Snapshot)data.GetValue(form)).job.state!="deferred")throw new Exception("Native defer failed");
   await (Task)send.Invoke(form,new object[]{"cancel",true});if(((Snapshot)data.GetValue(form)).job.state!="cancelled")throw new Exception("Native cancel failed");
   File.WriteAllText(args[1],"PASS: native HTTP credential / JSON / failed restore metadata / defer / cancel / automatic close / topmost");result=0;
  }catch(Exception ex){File.WriteAllText(args[1],ex.ToString());}finally{if(result!=0)form.Close();}};
  Application.Run(form);if(!form.IsDisposed)throw new Exception("Terminal prompt did not close");
 }return result;}
}
