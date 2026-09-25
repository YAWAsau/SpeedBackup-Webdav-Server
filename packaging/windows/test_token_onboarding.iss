#define ONBOARDING_TEST
[Setup]
AppName=SpeedBackup onboarding test
AppVersion=1
CreateAppDir=no
Uninstallable=no
PrivilegesRequired=lowest
OutputDir=..\..\dist\onboarding-test
OutputBaseFilename=onboarding-test
WizardStyle=modern dynamic windows11

[Code]
function DataDirectory(Param: String): String;
begin
  Result := ExpandConstant('{param:FIXTURE}');
end;
function ServerPort(Param: String): String;
begin
  Result := ExpandConstant('{param:PORT}');
end;
function AdminURL(Param: String): String;
begin
  Result := 'http://127.0.0.1:' + ServerPort('') + '/web/admin';
end;
#include "token_onboarding.iss"
procedure InitializeWizard();
begin
  CreateLoginControls();
end;
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep <> ssPostInstall then Exit;
  ShowLoginControls();
  if not OpenAdminButton.Visible then RaiseException('Open admin control missing');
  if LoginHelp.Height <= 0 then RaiseException('Help outside completion page');
  if Pos('admin-reset', LoginHelp.Caption) = 0 then RaiseException('Recovery guidance missing');
  SaveStringToFile(DataDirectory('') + '\onboarding-test.txt', 'PASS: account onboarding and local recovery controls', False);
end;
