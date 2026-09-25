; Service + ProgramData lifecycle inspired by SFTPGo; operations are checked.
#ifndef MyAppName
  #define MyAppName "SpeedBackup Server"
#endif
#ifndef MyAppId
  #define MyAppId "{{A6FBD59E-1B48-4F89-9F4B-7C6D40D9B841}"
#endif
#ifndef MyAppVersion
  #define MyAppVersion "0.3.10-webdav11"
#endif
#ifndef MyServiceName
  #define MyServiceName "SpeedBackupServer"
#endif
#ifndef MyBinaryDir
  #define MyBinaryDir "..\..\dist\windows-amd64"
#endif
#ifndef MyOutputDir
  #define MyOutputDir "..\..\dist\windows-setup"
#endif
#ifndef MyPickerProtocol
  #define MyPickerProtocol "speedbackup-picker"
#endif
#define MyAppExeName "speedbackup-server.exe"
#define MyRegKey "Software\" + MyAppName

[Setup]
AppId={#MyAppId}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher=SpeedBackup
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir={#MyOutputDir}
OutputBaseFilename=SpeedBackup_Server_Setup_v{#MyAppVersion}_windows_x64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern dynamic windows11
WizardSizePercent=115,115
WizardImageFile=assets\welcome.png
WizardSmallImageFile=assets\mark.png
SetupIconFile=assets\server.ico
DisableWelcomePage=no
#ifdef OPTIONS_TEST
PrivilegesRequired=lowest
CreateAppDir=no
Uninstallable=no
#else
PrivilegesRequired=admin
#endif
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
UninstallDisplayIcon={app}\{#MyAppExeName}
CloseApplications=no
RestartIfNeededByRun=no
SetupLogging=yes

[Languages]
Name: "zhTW"; MessagesFile: "Languages\ChineseTraditional.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

#ifndef OPTIONS_TEST
[Files]
Source: "{#MyBinaryDir}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#MyBinaryDir}\speedbackup-picker.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\THIRD_PARTY_NOTICES.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\reset_admin.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "README_WINDOWS_INSTALLER.md"; DestDir: "{app}"; Flags: ignoreversion

[Tasks]
Name: "firewall"; Description: "允許同一區網裝置連線及自動發現（本程式的 TCP 連接埠與 UDP 5353）"; GroupDescription: "網路存取"

[Icons]
Name: "{group}\開啟管理介面"; Filename: "{code:AdminURL}"
Name: "{group}\伺服器資料"; Filename: "{code:DataDirectory}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

[Run]
; Login actions are shown together with the token on the completion page.

[UninstallRun]
Filename: "{app}\{#MyAppExeName}"; Parameters: "service uninstall"; Flags: runhidden waituntilterminated; RunOnceId: "Uninstall service"
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""{#MyAppName}"""; Flags: runhidden waituntilterminated; RunOnceId: "Remove firewall rule"

[Registry]
Root: HKLM; Subkey: "Software\Classes\{#MyPickerProtocol}"; ValueType: string; ValueName: ""; ValueData: "URL:SpeedBackup folder picker"; Flags: uninsdeletekey
Root: HKLM; Subkey: "Software\Classes\{#MyPickerProtocol}"; ValueType: string; ValueName: "URL Protocol"; ValueData: ""
Root: HKLM; Subkey: "Software\Classes\{#MyPickerProtocol}\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\speedbackup-picker.exe"" ""%1"""
Root: HKLM; Subkey: "{#MyRegKey}"; ValueType: string; ValueName: "DataDir"; ValueData: "{code:DataDirectory}"
Root: HKLM; Subkey: "{#MyRegKey}"; ValueType: string; ValueName: "Port"; ValueData: "{code:ServerPort}"
#endif

[Messages]
zhTW.WelcomeLabel1=讓備份有一個安心的家
zhTW.WelcomeLabel2=此精靈將安裝 SpeedBackup Server 背景服務，可選擇是否開機自動啟動。%n%n首次安裝可選資料位置；升級沿用原位置。安裝完成後，可從瀏覽器管理備份。%n%n升級會保留既有資料與帳號設定。
zhTW.FinishedHeadingLabel=伺服器已準備就緒
zhTW.FinishedLabel=服務已成功啟動。請妥善保存首次產生的存取憑證，再開啟管理介面。%n%n解除安裝只會移除程式與服務，備份資料會保留。
en.FinishedLabel=The service has started. Save the first-run token before opening WebAdmin. Uninstalling preserves your backup data.

[Code]
var
  StoragePage: TInputDirWizardPage;
  NetworkPage: TInputQueryWizardPage;
  ExistingData: String;
  InstalledOK: Boolean;
  AutoStartCheck: TNewCheckBox;
  AutoStartHelp: TNewStaticText;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := (PageID = StoragePage.ID) and (ExistingData <> '');
end;

function AutoStartValue(): String;
begin
  if AutoStartCheck.Checked then Result := 'true' else Result := 'false';
end;

function DataDirectory(Param: String): String;
begin
  Result := RemoveBackslashUnlessRoot(StoragePage.Values[0]);
end;
function ServerPort(Param: String): String;
begin
  Result := NetworkPage.Values[0];
end;
function AdminURL(Param: String): String;
begin
  Result := 'http://127.0.0.1:' + ServerPort('') + '/web/admin';
end;
#include "token_onboarding.iss"
function CheckedExec(FileName, Parameters, Step: String): String;
var Code: Integer;
begin
  Result := '';
  WizardForm.StatusLabel.Caption := Step;
  if not Exec(FileName, Parameters, '', SW_HIDE, ewWaitUntilTerminated, Code) then
    Result := Step + ': 無法執行程式。'
  else if Code <> 0 then
    Result := Step + ': 執行失敗，錯誤碼 ' + IntToStr(Code) + '。';
end;
procedure MustRun(FileName, Parameters, Step: String);
var Error: String;
begin
  Error := CheckedExec(FileName, Parameters, Step);
  if Error <> '' then RaiseException(Error);
end;
procedure InitializeWizard();
var Port, RequestedData, Startup: String; StartType: Cardinal;
begin
  InstalledOK := False;
  ExistingData := '';
  #ifdef OPTIONS_TEST
  ExistingData := ExpandConstant('{param:EXISTINGDATA|}');
  #else
  RegQueryStringValue(HKLM, '{#MyRegKey}', 'DataDir', ExistingData);
  #endif
  StoragePage := CreateInputDirPage(wpSelectDir, '選擇備份儲存位置',
    '程式與備份資料分開存放', '升級與解除安裝都會保留這裡的資料。請選擇空間足夠的本機磁碟。', False, '');
  StoragePage.Add('資料資料夾：');
  if ExistingData = '' then
    StoragePage.Values[0] := ExpandConstant('{param:DATA|{commonappdata}\{#MyAppName}}')
  else begin
    StoragePage.Values[0] := ExistingData;
    StoragePage.Edits[0].Enabled := False;
    StoragePage.Buttons[0].Enabled := False;
    RequestedData := ExpandConstant('{param:DATA|}');
    if (RequestedData <> '') and (CompareText(RequestedData, ExistingData) <> 0) then
      RaiseException('升級不能直接更換資料位置。請先完成資料遷移。');
  end;
  NetworkPage := CreateInputQueryPage(StoragePage.ID, '設定連線',
    '讓手機與管理介面連上伺服器', '預設連接埠為 8765。若已被其他程式使用，請改用其他連接埠。');
  NetworkPage.Add('連接埠（1–65535）：', False);
  Port := '8765';
  RegQueryStringValue(HKLM, '{#MyRegKey}', 'Port', Port);
  NetworkPage.Values[0] := ExpandConstant('{param:PORT|' + Port + '}');
  AutoStartCheck := TNewCheckBox.Create(WizardForm);
  AutoStartCheck.Parent := NetworkPage.Surface;
  AutoStartCheck.Caption := '開機自動啟動 SpeedBackup Server';
  AutoStartCheck.SetBounds(0, NetworkPage.Edits[0].Top + NetworkPage.Edits[0].Height + ScaleY(22), NetworkPage.SurfaceWidth, ScaleY(24));
  AutoStartCheck.Checked := True;
  if RegQueryDWordValue(HKLM, 'SYSTEM\CurrentControlSet\Services\{#MyServiceName}', 'Start', StartType) then
    AutoStartCheck.Checked := StartType = 2;
  Startup := Lowercase(ExpandConstant('{param:AUTOSTART|}'));
  if Startup <> '' then begin
    if (Startup <> 'true') and (Startup <> 'false') then RaiseException('/AUTOSTART must be true or false');
    AutoStartCheck.Checked := Startup = 'true';
  end;
  AutoStartHelp := TNewStaticText.Create(WizardForm);
  AutoStartHelp.Parent := NetworkPage.Surface;
  AutoStartHelp.AutoSize := False; AutoStartHelp.WordWrap := True;
  AutoStartHelp.SetBounds(0, AutoStartCheck.Top + ScaleY(30), NetworkPage.SurfaceWidth, ScaleY(48));
  AutoStartHelp.Caption := '安裝完成後會先啟動服務。此選項控制日後開機是否自動啟動；升級預設沿用目前設定。';
  CreateLoginControls();
end;
procedure CurPageChanged(CurPageID: Integer);
begin
  if (CurPageID = wpFinished) and InstalledOK then ShowLoginControls();
end;
function NextButtonClick(CurPageID: Integer): Boolean;
var Port: Integer;
begin
  Result := True;
  if CurPageID = NetworkPage.ID then begin
    Port := StrToIntDef(ServerPort(''), 0);
    Result := (Port >= 1) and (Port <= 65535);
    if not Result then MsgBox('請輸入 1 到 65535 之間的連接埠。', mbError, MB_OK);
  end;
end;
function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo, MemoTypeInfo,
  MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
begin
  Result := '程式位置' + NewLine + Space + ExpandConstant('{app}') + NewLine + NewLine +
    '備份儲存位置' + NewLine + Space + DataDirectory('') + NewLine + NewLine +
    '管理介面' + NewLine + Space + AdminURL('') + NewLine + NewLine +
    '開機自動啟動：' + AutoStartValue() + NewLine + NewLine + MemoTasksInfo;
end;
function PrepareToInstall(var NeedsRestart: Boolean): String;
var Port: Integer; Data: String; Entry: TFindRec;
begin
  Result := '';
  #ifdef OPTIONS_TEST
  Exit;
  #endif
  Port := StrToIntDef(ServerPort(''), 0);
  if (Port < 1) or (Port > 65535) then begin
    Result := '連接埠必須介於 1 到 65535。'; Exit;
  end;
  Data := DataDirectory('');
  if (Pos('"', Data) > 0) or (Length(Data) <= 3) then begin
    Result := '資料路徑無效。'; Exit;
  end;
  if (Data[2] <> ':') or (Data[3] <> '\') then begin
    Result := '請選擇本機磁碟中的專用資料夾。'; Exit;
  end;
  if DirExists(Data) and not DirExists(Data + '\data\.speedbackup-server') then begin
    if FindFirst(Data + '\*', Entry) then begin
      try
        repeat
          if (Entry.Name <> '.') and (Entry.Name <> '..') then begin
            Result := '請選擇空資料夾，或既有的 SpeedBackup Server 資料位置。'; Exit;
          end;
        until not FindNext(Entry);
      finally FindClose(Entry); end;
    end;
  end;
  { Use the NEW helper before replacing files: go5 stop returned too early. }
  ExtractTemporaryFile('{#MyAppExeName}');
  Result := CheckedExec(ExpandConstant('{tmp}\{#MyAppExeName}'), 'service stop', '停止舊版服務');
end;
procedure CurStepChanged(CurStep: TSetupStep);
var Exe, Data, Root, Token: String; IsNewData: Boolean;
begin
  if CurStep = ssPostInstall then begin
    #ifdef OPTIONS_TEST
    if (ExistingData = '') and (ShouldSkipPage(StoragePage.ID) or not StoragePage.Buttons[0].Enabled or not StoragePage.Edits[0].Enabled) then
      RaiseException('Fresh installation must offer editable data directory');
    if (ExistingData <> '') and not ShouldSkipPage(StoragePage.ID) then RaiseException('Upgrade must skip data directory page');
    if not AutoStartCheck.Enabled then RaiseException('Startup choice is disabled');
    if Lowercase(ExpandConstant('{param:AUTOSTART|true}')) <> AutoStartValue() then RaiseException('Startup choice mismatch');
    if not SaveStringToFile(ExpandConstant('{param:RESULTFILE}'), 'PASS: existing=' + ExistingData + '; startup=' + AutoStartValue(), False) then RaiseException('Cannot write test result');
    InstalledOK := True;
    Exit;
    #endif
    Exe := ExpandConstant('{app}\{#MyAppExeName}');
    Data := DataDirectory(''); Root := Data + '\data'; Token := Data + '\FIRST_RUN_TOKEN.txt';
    IsNewData := not DirExists(Root + '\.speedbackup-server');
    if not ForceDirectories(Data) then RaiseException('無法建立備份資料夾。');
    if IsNewData then
      MustRun(ExpandConstant('{sys}\icacls.exe'), '"' + Data + '" /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F"', '設定資料存取權限');
    if not ForceDirectories(Root) then RaiseException('無法建立備份資料夾。');
    MustRun(Exe, 'init --root "' + Root + '" --token-file "' + Token + '"', '初始化存取憑證');
    MustRun(Exe, 'service install --root "' + Root + '" --listen "0.0.0.0:' + ServerPort('') + '" --token-file "' + Token + '"', '註冊背景服務');
    MustRun(Exe, 'service autostart --enabled=' + AutoStartValue(), '設定開機啟動選項');
    if WizardIsTaskSelected('firewall') then begin
      { Deleting a nonexistent rule is harmless and returns nonzero on Windows. }
      CheckedExec(ExpandConstant('{sys}\netsh.exe'), 'advfirewall firewall delete rule name="{#MyAppName}"', '更新區網規則');
      MustRun(ExpandConstant('{sys}\netsh.exe'), 'advfirewall firewall add rule name="{#MyAppName}" dir=in action=allow program="' + Exe + '" enable=yes profile=any protocol=TCP localport=' + ServerPort('') + ' remoteip=localsubnet', '允許同一區網連線');
      MustRun(ExpandConstant('{sys}\netsh.exe'), 'advfirewall firewall add rule name="{#MyAppName}" dir=in action=allow program="' + Exe + '" enable=yes profile=any protocol=UDP localport=5353 remoteip=localsubnet', '允許同一區網自動發現');
    end;
    MustRun(Exe, 'service start', '啟動服務並確認狀態');
    InstalledOK := True;
    WizardForm.FinishedLabel.Caption := '服務已啟動。' + #13#10#13#10 +
      '管理介面：' + AdminURL('') + #13#10#13#10 +
      '首次存取憑證：' + Token + #13#10#13#10 + '解除安裝會保留備份資料。';
  end;
end;
function GetCustomSetupExitCode(): Integer;
begin
  if InstalledOK then Result := 0 else Result := 1;
end;
