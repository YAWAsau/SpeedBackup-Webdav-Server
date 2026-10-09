// Native Windows desktop prompt. No browser, WebView or external UI framework.
// Build with the Windows .NET Framework compiler; OPPO font is embedded unmodified.
using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Drawing.Text;
using System.IO;
using System.Net;
using System.Net.Http;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows.Forms;
using Microsoft.Win32;

namespace SpeedBackupDesktop {
 public sealed class Job {public string job_id;public string outcome;public string operation;public string state;public string message;public bool preview;public long deadline_ms;}
 public sealed class Device {public string id;public string name;public bool approved;public bool revoked;}
 public sealed class Snapshot {public Device device;public Dictionary<string,string> theme;public Job job;public int active_requests;public long server_ms;}
 public sealed class PromptButton : Control {
  public Color Ink=Color.Black,Fill=Color.White,Edge=Color.White;public bool ChromeButton,CloseIcon;bool hovered;
  public PromptButton(){SetStyle(ControlStyles.UserPaint|ControlStyles.AllPaintingInWmPaint|ControlStyles.OptimizedDoubleBuffer|ControlStyles.Opaque|ControlStyles.Selectable|ControlStyles.ResizeRedraw,true);Cursor=Cursors.Hand;TabStop=true;AccessibleRole=AccessibleRole.PushButton;}
  internal void PaintProbe(Graphics graphics){OnPaint(new PaintEventArgs(graphics,ClientRectangle));}
  public void PerformClick(){if(Enabled)OnClick(EventArgs.Empty);}
  protected override void OnMouseDown(MouseEventArgs e){if(e.Button==MouseButtons.Left)Focus();base.OnMouseDown(e);}
  protected override void OnMouseEnter(EventArgs e){hovered=true;Invalidate();base.OnMouseEnter(e);}
  protected override void OnMouseLeave(EventArgs e){hovered=false;Invalidate();base.OnMouseLeave(e);}
  protected override void OnGotFocus(EventArgs e){Invalidate();base.OnGotFocus(e);}
  protected override void OnLostFocus(EventArgs e){Invalidate();base.OnLostFocus(e);}
  protected override bool IsInputKey(Keys keys){var key=keys&Keys.KeyCode;return key==Keys.Enter||key==Keys.Space||base.IsInputKey(keys);}
  protected override void OnKeyDown(KeyEventArgs e){if(e.KeyCode==Keys.Enter||e.KeyCode==Keys.Space){PerformClick();e.Handled=true;e.SuppressKeyPress=true;}base.OnKeyDown(e);}
  protected override AccessibleObject CreateAccessibilityInstance(){return new ButtonAccessibility(this);}
  sealed class ButtonAccessibility : ControlAccessibleObject {readonly PromptButton button;public ButtonAccessibility(PromptButton b):base(b){button=b;}public override string DefaultAction{get{return "按下";}}public override void DoDefaultAction(){button.PerformClick();}}
  protected override void OnResize(EventArgs e){base.OnResize(e);if(Width<8||Height<8)return;using(var path=Prompt.Round(new RectangleF(0,0,Width,Height),Math.Min(Height/2f,26))){var old=Region;Region=new Region(path);if(old!=null)old.Dispose();}}
  protected override void OnPaintBackground(PaintEventArgs e){}
  protected override void OnPaint(PaintEventArgs e){
   // Paint every pixel in WM_PAINT itself; native Button transparent erasure can
   // leave a black backbuffer on a real desktop even when WM_PRINT looks correct.
   var owner=Parent as Prompt;if(owner!=null){var state=e.Graphics.Save();e.Graphics.TranslateTransform(-Left,-Top);owner.PaintSurface(e.Graphics);e.Graphics.Restore(state);}else e.Graphics.Clear(Fill);
   var g=e.Graphics;g.SmoothingMode=SmoothingMode.AntiAlias;g.TextRenderingHint=TextRenderingHint.AntiAliasGridFit;
   using(var p=Prompt.Round(new RectangleF(1,1,Width-3,Height-3),Math.Min(Height/2f-3,26)))using(var b=new SolidBrush(ChromeButton?Color.FromArgb(hovered?28:0,Ink):Fill))using(var pen=new Pen(Edge,1)){g.FillPath(b,p);if(!ChromeButton)g.DrawPath(pen,p);}
   if(ChromeButton){float cx=Width/2f,cy=Height/2f,r=Width*.13f;using(var pen=new Pen(Ink,Math.Max(1.5f,Width/28f))){pen.StartCap=pen.EndCap=LineCap.Round;if(CloseIcon){g.DrawLine(pen,cx-r,cy-r,cx+r,cy+r);g.DrawLine(pen,cx+r,cy-r,cx-r,cy+r);}else g.DrawLine(pen,cx-r,cy,cx+r,cy);}}
   else using(var brush=new SolidBrush(Enabled?Ink:Color.Gray))using(var fmt=new StringFormat{Alignment=StringAlignment.Center,LineAlignment=StringAlignment.Center})g.DrawString(Text,Font,brush,ClientRectangle,fmt);
   if(Focused&&ShowFocusCues)using(var p=Prompt.Round(new RectangleF(4,4,Width-9,Height-9),Math.Min(Height/2f-6,23)))using(var pen=new Pen(Ink,1.5f))g.DrawPath(pen,p);
  }
 }
 public sealed class Prompt : Form {
  [DllImport("user32.dll")]static extern bool ReleaseCapture();
  [DllImport("user32.dll")]static extern bool SetForegroundWindow(IntPtr hwnd);
  [DllImport("user32.dll")]static extern IntPtr SendMessage(IntPtr hwnd,int msg,IntPtr wparam,IntPtr lparam);
  [DllImport("gdi32.dll")]static extern IntPtr AddFontMemResourceEx(IntPtr data,uint size,IntPtr reserved,ref uint count);
  [DllImport("gdi32.dll")]static extern bool RemoveFontMemResourceEx(IntPtr font);
  [DllImport("dwmapi.dll")]static extern int DwmSetWindowAttribute(IntPtr hwnd,int attr,ref int value,int size);
  readonly PrivateFontCollection fonts=new PrivateFontCollection();IntPtr fontMemory,fontHandle;
  readonly HttpClient client;readonly JavaScriptSerializer json=new JavaScriptSerializer();readonly string endpoint,token;
  readonly Timer heartbeat=new Timer{Interval=2000},clock=new Timer{Interval=250};
  readonly PromptButton now=new PromptButton(),later=new PromptButton(),cancel=new PromptButton(),minimize=new PromptButton(),dismiss=new PromptButton();
  readonly bool demo;bool pairing,nativeCorners;bool busy,closing,allowClose,locked,offline,failing,exitQueued;Size outlineSize,layoutSize;bool outlineNative;string appliedFrame;internal bool SuppressAutoClose;Snapshot data;long offset;string message="正在連接伺服器…";Color background,ink,muted,accent;bool dark,glass;
  Font heading,body,large,small,buttonFont;
  public static long Millis(){return (long)(DateTime.UtcNow-new DateTime(1970,1,1,0,0,0,DateTimeKind.Utc)).TotalMilliseconds;}
  public static GraphicsPath Round(RectangleF r,float radius){var p=new GraphicsPath();float d=radius*2;p.AddArc(r.X,r.Y,d,d,180,90);p.AddArc(r.Right-d,r.Y,d,d,270,90);p.AddArc(r.Right-d,r.Bottom-d,d,d,0,90);p.AddArc(r.X,r.Bottom-d,d,d,90,90);p.CloseFigure();return p;}
  public Prompt(string address,bool preview){
   demo=preview;AutoScaleMode=AutoScaleMode.None;ClientSize=new Size(800,560);StartPosition=FormStartPosition.CenterScreen;Text="SpeedBackup · 備份後關機";FormBorderStyle=FormBorderStyle.None;MaximizeBox=false;ShowIcon=false;TopMost=true;WindowState=FormWindowState.Normal;
   SetStyle(ControlStyles.AllPaintingInWmPaint|ControlStyles.UserPaint|ControlStyles.OptimizedDoubleBuffer,true);
   using(var stream=Assembly.GetExecutingAssembly().GetManifestResourceStream("OPPO.ttf")){if(stream==null)throw new InvalidOperationException("Bundled font missing");byte[] bytes=new byte[stream.Length];int read=0;while(read<bytes.Length){int n=stream.Read(bytes,read,bytes.Length-read);if(n==0)throw new EndOfStreamException();read+=n;}fontMemory=Marshal.AllocHGlobal(bytes.Length);Marshal.Copy(bytes,0,fontMemory,bytes.Length);uint count=0;fontHandle=AddFontMemResourceEx(fontMemory,(uint)bytes.Length,IntPtr.Zero,ref count);fonts.AddMemoryFont(fontMemory,bytes.Length);}
   var family=fonts.Families[0];heading=new Font(family,28,FontStyle.Regular,GraphicsUnit.Pixel);body=new Font(family,18,FontStyle.Regular,GraphicsUnit.Pixel);large=new Font(family,72,FontStyle.Regular,GraphicsUnit.Pixel);small=new Font(family,15,FontStyle.Regular,GraphicsUnit.Pixel);Font=body;
   foreach(var b in new[]{now,later,cancel}){b.Font=body;b.Enabled=false;Controls.Add(b);}
   minimize.Text="最小化";dismiss.Text="關閉";dismiss.CloseIcon=true;minimize.AccessibleName="最小化並暫停倒數";dismiss.AccessibleName="關閉並暫停倒數";minimize.TabIndex=3;dismiss.TabIndex=4;
   foreach(var b in new[]{minimize,dismiss}){b.ChromeButton=true;b.Font=body;Controls.Add(b);}minimize.Click+=(s,e)=>WindowState=FormWindowState.Minimized;dismiss.Click+=(s,e)=>Close();
   MouseDown+=(s,e)=>{if(e.Button==MouseButtons.Left&&e.Y<90*ClientSize.Width/800f){ReleaseCapture();SendMessage(Handle,0xA1,new IntPtr(2),IntPtr.Zero);}};
   now.Text="立即關機";later.Text="延後 10 分鐘";cancel.Text="取消關機";now.TabIndex=2;later.TabIndex=1;cancel.TabIndex=0;
   now.Click+=async(s,e)=>await Send(pairing?"approve":"now",true);later.Click+=async(s,e)=>await Send("defer",true);cancel.Click+=async(s,e)=>await Send(pairing?"reject":"cancel",true);
   heartbeat.Tick+=async(s,e)=>{if(!locked&&WindowState!=FormWindowState.Minimized&&!closing&&!Terminal())await Send("heartbeat");};clock.Tick+=(s,e)=>Invalidate();
   if(!demo){Uri u;if(!Uri.TryCreate(address,UriKind.Absolute,out u)||u.Scheme!="http"||u.Host!="127.0.0.1"||u.UserInfo!=""||u.Query!=""||(u.AbsolutePath!="/api/v1/power/desktop"&&u.AbsolutePath!="/api/v1/power/device-desktop")||!System.Text.RegularExpressions.Regex.IsMatch(u.Fragment,"^#sb1_[a-f0-9]{64}$"))throw new ArgumentException("Invalid local desktop capability");pairing=u.AbsolutePath=="/api/v1/power/device-desktop";endpoint=u.GetLeftPart(UriPartial.Authority)+u.AbsolutePath; if(pairing){Text="SpeedBackup · 裝置授權";TopMost=true;};token=u.Fragment.Substring(1);client=new HttpClient(new HttpClientHandler{UseProxy=false,AllowAutoRedirect=false}){Timeout=TimeSpan.FromSeconds(4)};}
   else {data=new Snapshot{theme=new Dictionary<string,string>{{"theme","white"},{"surface","glass"},{"accent","blue"}},job=new Job{job_id="local-preview",state="countdown",preview=true,deadline_ms=Millis()+60000,message="請儲存工作，電腦即將正常關機"}};}
   Apply();Resize+=async(s,e)=>{LayoutButtons();if(WindowState==FormWindowState.Minimized)await Send("pause",true);else if(!locked)await Send("heartbeat");};
   Shown+=async(s,e)=>{if(ShowInTaskbar){WindowState=FormWindowState.Normal;BringToFront();Activate();SetForegroundWindow(Handle);}cancel.Focus();heartbeat.Start();clock.Start();await Send("heartbeat");};
   FormClosing+=async(s,e)=>{if(allowClose)return;e.Cancel=true;if(closing)return;closing=true;heartbeat.Stop();await Send("pause",true);allowClose=true;Close();};
   SystemEvents.SessionSwitch+=SessionChanged;LayoutButtons();
  }
  // Let DWM own the smooth outer silhouette on Windows 11. A GDI Region is
  // binary (not antialiased) and also prevents DWM from rounding the window.
  protected override void OnHandleCreated(EventArgs e){base.OnHandleCreated(e);ConfigureFrame();}
  void ConfigureFrame(){if(!IsHandleCreated)return;try{int border=unchecked((int)0xFFFFFFFE),corner=2;int b=DwmSetWindowAttribute(Handle,34,ref border,4);nativeCorners=b==0&&DwmSetWindowAttribute(Handle,33,ref corner,4)==0;if(!nativeCorners){int disabled=1;DwmSetWindowAttribute(Handle,2,ref disabled,4);}}catch{nativeCorners=false;}UpdateOutline();}
  void UpdateOutline(){if(IsDisposed||Disposing||ClientSize.Width<1||ClientSize.Height<1)return;if(outlineSize==ClientSize&&outlineNative==nativeCorners)return;outlineSize=ClientSize;outlineNative=nativeCorners;var old=Region;if(nativeCorners){if(Region!=null)Region=null;}else using(var outline=Round(new RectangleF(0,0,ClientSize.Width,ClientSize.Height),12*ClientSize.Width/800f)){Region=new Region(outline);}if(old!=null)old.Dispose();}
  protected override void WndProc(ref Message m){base.WndProc(ref m);if(m.Msg==0x31E)ConfigureFrame();}
  protected override bool ProcessCmdKey(ref Message msg,Keys keyData){if(keyData==Keys.Escape){Close();return true;}return base.ProcessCmdKey(ref msg,keyData);}
  bool Terminal(){return data!=null&&data.job!=null&&data.job.state!="waiting"&&data.job.state!="countdown"&&data.job.state!="deferred";}
  string Theme(string key,string fallback){string v;return data!=null&&data.theme!=null&&data.theme.TryGetValue(key,out v)?v:fallback;}
  protected override void OnLoad(EventArgs e){using(var graphics=CreateGraphics()){float dpi=graphics.DpiX/96f;ClientSize=new Size((int)(800*dpi),(int)(560*dpi));}if(StartPosition!=FormStartPosition.Manual){var area=Screen.FromControl(this).WorkingArea;Location=new Point(area.Left+(area.Width-Width)/2,area.Top+(area.Height-Height)/2);}base.OnLoad(e);}
  void LayoutButtons(){if(IsDisposed||Disposing||WindowState==FormWindowState.Minimized||layoutSize==ClientSize)return;layoutSize=ClientSize;float scale=ClientSize.Width/800f;UpdateOutline();int width=(int)(224*scale),gap=(int)(16*scale),left=(int)(48*scale),top=(int)(420*scale);var previous=buttonFont;buttonFont=new Font(body.FontFamily,18*scale,FontStyle.Regular,GraphicsUnit.Pixel);int i=0;foreach(var b in new[]{now,later,cancel}){b.Font=buttonFont;b.SetBounds(left+i++*(width+gap),top,width,(int)(56*scale));}minimize.Font=dismiss.Font=buttonFont;minimize.SetBounds((int)(676*scale),(int)(25*scale),(int)(44*scale),(int)(44*scale));dismiss.SetBounds((int)(732*scale),(int)(25*scale),(int)(44*scale),(int)(44*scale));if(previous!=null)previous.Dispose();}
  void Apply(){
   if(IsDisposed||Disposing)return;
   string theme=Theme("theme","system");dark=theme=="dark"||theme=="black";if(theme=="system"){using(var k=Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize")){dark=k!=null&&Convert.ToInt32(k.GetValue("AppsUseLightTheme",1))==0;}}
   glass=Theme("surface","classic")=="glass";background=dark?(theme=="black"?Color.Black:Color.FromArgb(20,27,38)):Color.FromArgb(244,246,248);ink=dark?Color.FromArgb(236,242,250):Color.FromArgb(24,33,47);muted=dark?Color.FromArgb(192,205,220):Color.FromArgb(72,87,108);
   string a=Theme("accent","blue");accent=a=="teal"?Color.FromArgb(14,130,123):a=="violet"?Color.FromArgb(124,67,205):Color.FromArgb(33,103,213);
   bool pending=data!=null&&data.job!=null&&!Terminal()&&!offline;
   now.Enabled=pending&&(pairing||data.active_requests==0&&data.job.state!="waiting");later.Enabled=cancel.Enabled=pending;
   now.Text=data!=null&&data.job!=null&&data.job.preview?"模擬立即關機":"立即關機";
   if(pairing){now.Text="允許此裝置";cancel.Text="拒絕";later.Visible=false;minimize.AccessibleName="最小化授權視窗";dismiss.AccessibleName="關閉（不授權）";}
   foreach(var b in new[]{now,later,cancel}){b.Ink=b==now?Color.White:ink;b.Fill=b==now?accent:Blend(background,Color.White,dark?0.10f:0.65f);b.Edge=Blend(b.Fill,Color.White,0.35f);b.Invalidate();}
   foreach(var b in new[]{minimize,dismiss}){b.Ink=ink;b.Fill=background;b.Edge=background;b.Invalidate();}
   if(data!=null&&data.job!=null&&!offline)message=data.job.message;
   string frameKey=dark.ToString();if(IsHandleCreated&&appliedFrame!=frameKey){appliedFrame=frameKey;int v=dark?1:0;try{DwmSetWindowAttribute(Handle,20,ref v,4);}catch{}ConfigureFrame();}
   Invalidate();
  }
  static Color Blend(Color a,Color b,float t){return Color.FromArgb((int)(a.R+(b.R-a.R)*t),(int)(a.G+(b.G-a.G)*t),(int)(a.B+(b.B-a.B)*t));}
  void SessionChanged(object sender,SessionSwitchEventArgs e){if(IsDisposed||Disposing||!IsHandleCreated)return;try{BeginInvoke((Action)(async()=>{if(e.Reason==SessionSwitchReason.SessionLock||e.Reason==SessionSwitchReason.ConsoleDisconnect||e.Reason==SessionSwitchReason.RemoteDisconnect){locked=true;await Send("pause",true);}else if(e.Reason==SessionSwitchReason.SessionUnlock||e.Reason==SessionSwitchReason.ConsoleConnect||e.Reason==SessionSwitchReason.RemoteConnect){locked=false;await Send("heartbeat");}}));}catch(InvalidOperationException){}}
  async Task Send(string action,bool urgent=false){
   if(IsDisposed||closing&&!urgent||locked&&action=="heartbeat")return;
   while(busy){if(!urgent)return;await Task.Delay(50);if(IsDisposed)return;}busy=true;
   try{
    if(demo){if(action=="cancel"){data.job.state="cancelled";data.job.message="使用者已取消關機";}if(action=="defer"){data.job.state="deferred";data.job.message="已延後 10 分鐘（安全預覽）";}if(action=="now"||data.job.state=="countdown"&&Millis()>=data.job.deadline_ms){data.job.state="simulated";data.job.message="預覽完成；沒有執行關機";}Apply();CloseIfFinished();return;}
    using(var request=new HttpRequestMessage(HttpMethod.Post,endpoint)){
     request.Headers.TryAddWithoutValidation("Authorization","Bearer "+token);request.Headers.TryAddWithoutValidation("X-SB-Admin","1");request.Content=new StringContent(json.Serialize(new{action=action,job_id=data!=null&&data.job!=null?data.job.job_id:""}),System.Text.Encoding.UTF8,"application/json");
     using(var response=await client.SendAsync(request)){if(!response.IsSuccessStatusCode)throw new IOException("Desktop disconnected");var text=await response.Content.ReadAsStringAsync();if(text.Length>16384)throw new IOException("Invalid response");var next=json.Deserialize<Snapshot>(text);if(next==null||next.job==null)throw new IOException("Missing job");if(IsDisposed)return;data=next;offset=data.server_ms-Millis();offline=false;Apply();CloseIfFinished();}
    }
   }catch(Exception error){if(!IsDisposed&&!Disposing){LogFailure(error);offline=true;message=pairing?"授權要求已失效或連線中斷，未批准裝置":"桌面提示連線中斷，倒數將暫停";Apply();}}finally{busy=false;}
  }
  void CloseIfFinished(){if(SuppressAutoClose||exitQueued||IsDisposed||Disposing||!Terminal())return;string state=data.job.state;if(state!="cancelled"&&state!="simulated"&&state!="approved"&&state!="rejected")return;exitQueued=true;heartbeat.Stop();clock.Stop();BeginInvoke((Action)(()=>{if(IsDisposed)return;allowClose=true;Close();}));}
  internal static void LogFailure(Exception error){try{string dir=Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"SpeedBackup");Directory.CreateDirectory(dir);File.WriteAllText(Path.Combine(dir,"desktop-last-error.log"),DateTime.UtcNow.ToString("o")+"\n"+error.GetType().FullName+"\n"+error.StackTrace);}catch{}}
  internal async void FailSafe(Exception error){LogFailure(error);if(failing||IsDisposed)return;failing=true;heartbeat.Stop();clock.Stop();Hide();try{await Send("pause",true);}catch(Exception nested){LogFailure(nested);}finally{allowClose=true;Close();}}
  internal void PaintSurface(Graphics g){var saved=g.Save();
   // Opaque, pixel-aligned coverage first. Antialiasing an origin-aligned
   // rectangle blends the first row/column with the default light backbuffer.
   g.SmoothingMode=SmoothingMode.None;using(var baseFill=new SolidBrush(background))g.FillRectangle(baseFill,ClientRectangle);g.ScaleTransform(ClientSize.Width/800f,ClientSize.Width/800f);
   if(glass){using(var b=new LinearGradientBrush(new Rectangle(0,0,800,560),Blend(background,accent,dark?.30f:.32f),Blend(background,Color.FromArgb(196,150,236),dark?.20f:.30f),35)){g.FillRectangle(b,0,0,800,560);}using(var path=new GraphicsPath()){path.AddEllipse(300,200,780,520);using(var radial=new PathGradientBrush(path)){radial.CenterColor=Color.FromArgb(dark?45:65,45,180,195);radial.SurroundColors=new[]{Color.Transparent};g.FillPath(radial,path);}}}g.Restore(saved);
  }
  protected override void OnPaint(PaintEventArgs e){
   var g=e.Graphics;g.SmoothingMode=SmoothingMode.AntiAlias;g.TextRenderingHint=TextRenderingHint.AntiAliasGridFit;PaintSurface(g);float scale=ClientSize.Width/800f;g.ScaleTransform(scale,scale);
   using(var brush=new SolidBrush(ink))using(var secondary=new SolidBrush(muted)){
    if(pairing){g.DrawString("SpeedBackup    裝置授權",body,brush,48,40);g.DrawString("允許這支手機請求關機？",heading,brush,48,100);g.DrawString(message,body,brush,new RectangleF(48,156,704,60));
     if(data!=null&&data.device!=null){g.DrawString("裝置："+data.device.name,body,brush,new RectangleF(48,228,704,44));string id=data.device.id;string fingerprint=id!=null&&id.Length==64?id.Substring(0,32)+"\n"+id.Substring(32):id;g.DrawString(fingerprint,body,brush,new RectangleF(48,284,704,76));}
     g.DrawString("請與手機上的完整指紋比對。允許後記住裝置，可於 WebUI 撤銷。",small,secondary,new RectangleF(48,365,704,42));g.DrawString("關閉視窗不會授權，也不會安排關機。",small,secondary,new RectangleF(48,503,704,38));return;}

    g.DrawString("SpeedBackup"+(data!=null&&data.job!=null&&data.job.preview?"    安全預覽":""),body,brush,48,40);string title="備份已完成";if(data!=null&&data.job!=null&&data.job.outcome!=null){title=(data.job.operation=="restore"?"恢復":"備份")+"已結束 · "+(data.job.outcome=="success"?"成功":data.job.outcome=="partial"?"部分完成":data.job.outcome=="cancelled"?"已取消":"失敗");}g.DrawString(title,heading,brush,48,100);
    g.DrawString(message,body,brush,new RectangleF(48,163,704,68));
    string count=offline?"暫停":data==null||data.job==null?"等待中":data.job.state=="countdown"?Math.Max(0,(long)Math.Ceiling((data.job.deadline_ms-Millis()-offset)/1000d))+" 秒":data.job.state=="deferred"?"已延後":Terminal()?"已結束":"等待中";
    g.DrawString(count,large,brush,48,231);g.DrawString("請先儲存工作。正常關機不會強制關閉尚未儲存的程式。",small,secondary,new RectangleF(48,363,704,42));
    g.DrawString("關閉或最小化視窗會暫停倒數。外觀使用 WebUI 同步的主題。",small,secondary,new RectangleF(48,503,704,38));
   }
  }
  protected override void Dispose(bool disposing){if(disposing){SystemEvents.SessionSwitch-=SessionChanged;heartbeat.Dispose();clock.Dispose();if(client!=null)client.Dispose();base.Dispose(true);heading.Dispose();if(buttonFont!=null)buttonFont.Dispose();body.Dispose();large.Dispose();small.Dispose();fonts.Dispose();if(fontHandle!=IntPtr.Zero)RemoveFontMemResourceEx(fontHandle);if(fontMemory!=IntPtr.Zero){Marshal.FreeHGlobal(fontMemory);fontMemory=IntPtr.Zero;}}else base.Dispose(false);}
  public void Render(string directory){SuppressAutoClose=true;Directory.CreateDirectory(directory);StartPosition=FormStartPosition.Manual;Location=new Point(-32000,-32000);ShowInTaskbar=false;Show();Location=new Point(-32000,-32000);heartbeat.Stop();clock.Stop();Application.DoEvents();foreach(string surface in new[]{"glass","classic"})foreach(string theme in new[]{"white","dark","black","system"}){data.theme["surface"]=surface;data.theme["theme"]=theme;Apply();if(nativeCorners?Region!=null:Region==null||Region.IsVisible(0,0)||!Region.IsVisible(Width/2,Height/2))throw new Exception("Window outline mode invalid");
   using(var expected=new Bitmap(Width,Height)){using(var g=Graphics.FromImage(expected))PaintSurface(g);foreach(var button in new[]{now,later,cancel,minimize,dismiss})using(var probe=new Bitmap(button.Width,button.Height)){using(var g=Graphics.FromImage(probe)){g.Clear(Color.Magenta);button.PaintProbe(g);}if(probe.GetPixel(0,0).ToArgb()!=expected.GetPixel(button.Left,button.Top).ToArgb())throw new Exception("Button backbuffer not fully painted");}}
   using(var bitmap=new Bitmap(Width,Height)){DrawToBitmap(bitmap,new Rectangle(0,0,Width,Height));if(surface=="classic"){foreach(var point in new[]{new Point(Width/2,0),new Point(0,Height/2),new Point(Width-1,Height/2),new Point(Width/2,Height-1)})if(bitmap.GetPixel(point.X,point.Y).ToArgb()!=background.ToArgb())throw new Exception("Outer edge is not fully opaque: "+point);}using(var clipped=new Bitmap(Width,Height)){using(var g=Graphics.FromImage(clipped)){if(Region!=null)g.SetClip(Region,CombineMode.Replace);g.DrawImageUnscaled(bitmap,0,0);}clipped.Save(Path.Combine(directory,"native-"+theme+(surface=="classic"?"-classic":"")+".png"));}}}later.PerformClick();if(data.job.state!="deferred")throw new Exception("Defer test failed");cancel.PerformClick();if(data.job.state!="cancelled")throw new Exception("Cancel test failed");data.job.state="countdown";Apply();now.PerformClick();if(data.job.state!="simulated")throw new Exception("Preview test failed");File.WriteAllText(Path.Combine(directory,"native-check.txt"),"Native WinForms / GDI+; no browser or WebView.\nFont: "+fonts.Families[0].Name+"\nFrame: "+(nativeCorners?"DWM rounded; border suppressed; no GDI region":"legacy region; nonclient rendering disabled")+"\nStyle: "+body.Style+"\nWindow outline / opaque button background: PASS\nOpaque outer edge pixels: PASS\nDefer / Cancel / Simulated shutdown: PASS");pairing=true;data.device=new Device{id=new string('a',64),name="測試手機 Xiaomi 13"};data.job.state="waiting";data.job.message="請核對手機顯示的裝置指紋，再允許此裝置請求關機。";data.theme["theme"]="dark";data.theme["surface"]="glass";Apply();using(var bitmap=new Bitmap(Width,Height)){DrawToBitmap(bitmap,new Rectangle(0,0,Width,Height));bitmap.Save(Path.Combine(directory,"native-device-pairing.png"));}allowClose=true;Hide();}
 }
 static class Program {
  [DllImport("user32.dll")]static extern bool SetProcessDPIAware();
  [STAThread]static int Main(string[] args){try{SetProcessDPIAware();Application.SetUnhandledExceptionMode(UnhandledExceptionMode.CatchException);Application.ThreadException+=(sender,eventArgs)=>{Prompt.LogFailure(eventArgs.Exception);foreach(Form window in Application.OpenForms){var prompt=window as Prompt;if(prompt!=null){prompt.FailSafe(eventArgs.Exception);return;}}Application.Exit();};Application.EnableVisualStyles();Application.SetCompatibleTextRenderingDefault(false);if(args.Length==2&&args[0]=="--render-preview"){using(var f=new Prompt(null,true))f.Render(args[1]);return 0;}if(args.Length!=1)return 2;Application.Run(new Prompt(args[0],args[0]=="--preview"));return 0;}catch{return 1;}}
 }
}
