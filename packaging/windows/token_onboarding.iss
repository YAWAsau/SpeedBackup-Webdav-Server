{ Account-based onboarding. No token copying. }
var
  OpenAdminButton: TNewButton;
  LoginHelp: TNewStaticText;
procedure OpenLoginPage(Sender: TObject);
var Code: Integer;
begin
  if not ShellExecAsOriginalUser('open', AdminURL(''), '', '', SW_SHOWNORMAL, ewNoWait, Code) then
    MsgBox('無法開啟瀏覽器，請手動開啟：' + AdminURL(''), mbError, MB_OK);
end;
procedure CreateLoginControls();
begin
  OpenAdminButton := TNewButton.Create(WizardForm);
  OpenAdminButton.Parent := WizardForm.FinishedLabel.Parent;
  OpenAdminButton.Caption := '開啟管理介面';
  OpenAdminButton.OnClick := @OpenLoginPage;
  OpenAdminButton.Visible := False;
  LoginHelp := TNewStaticText.Create(WizardForm);
  LoginHelp.Parent := OpenAdminButton.Parent;
  LoginHelp.AutoSize := False;
  LoginHelp.WordWrap := True;
  LoginHelp.Visible := False;
end;
procedure ShowLoginControls();
var X, Y, W: Integer;
begin
  #ifndef ONBOARDING_TEST
  if WizardSilent then Exit;
  #endif
  WizardForm.RunList.Visible := False;
  WizardForm.FinishedLabel.Caption := '開啟管理介面，首次使用時建立管理員帳號與密碼。' + #13#10 +
    '往後直接以帳號密碼登入，不必保存登入 Token。';
  WizardForm.FinishedLabel.Height := ScaleY(48);
  X := WizardForm.FinishedLabel.Left; W := WizardForm.FinishedLabel.Width;
  Y := WizardForm.FinishedLabel.Top + WizardForm.FinishedLabel.Height + ScaleY(12);
  OpenAdminButton.SetBounds(X, Y, W, ScaleY(34)); OpenAdminButton.Visible := True;
  LoginHelp.SetBounds(X, Y + ScaleY(48), W, LoginHelp.Parent.ClientHeight - Y - ScaleY(54));
  LoginHelp.Caption := '管理介面：' + AdminURL('') + #13#10#13#10 +
    '升級保留管理員與備份設定；從舊 Token 版本升級時，第一次在本機建立管理員即可。' + #13#10#13#10 +
    '忘記密碼可使用本機 admin-reset 命令重設，備份資料不會刪除。';
  LoginHelp.Visible := True;
end;
