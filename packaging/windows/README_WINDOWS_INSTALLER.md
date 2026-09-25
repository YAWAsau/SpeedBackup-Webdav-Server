# Windows 安裝

Setup使用Inno Setup，安裝完成後按「開啟管理介面」。第一次在localhost建立管理員，之後用帳號密碼登入。舊Token版升級不需找回Token。

預設程式：C:\Program Files\SpeedBackup Server
預設資料：C:\ProgramData\SpeedBackup Server\data
管理頁：http://127.0.0.1:8765/web/admin

分享目錄可在網頁直接輸入或瀏覽选择。升級保留設定與資料，卸載預設保留備份。忘記管理密碼可停止服務後執行reset_admin.ps1；詳見主README。API Token檔案保留給既有API客戶端，網頁登入不需要它。

首次安裝顯示可編輯的資料目錄；已有安裝時沿用原位置並跳過該頁。連線頁可勾選「開機自動啟動」，升級預設沿用 SCM 目前設定，靜默安裝用 /AUTOSTART=true 或 false。安裝後仍會先啟動本次服務。

區網防火牆選項限制為此安裝程式的 TCP 連接埠與 LocalSubnet 來源，涵蓋公用／私人／網域網路。升級勾選時更新舊規則，不會關閉 Windows 防火牆。
