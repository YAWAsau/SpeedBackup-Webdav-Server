# SpeedBackup Webdav Server v0.3.14-webdav15


Windows / Linux 開源 WebDAV 備份伺服器，提供管理網頁、帳號與目錄管理、即時傳輸監看及內網 mDNS 公告。

- [下載 Windows / Linux Releases](https://github.com/YAWAsau/SpeedBackup-Webdav-Server/releases)
- 本儲存庫僅包含伺服器，不包含 Android 工具或私人測試資料。
- Windows x64：安裝版或可攜版；Linux amd64 / arm64：可攜包。
- 完整原始碼包含 Go vendor 依賴；建置要求 Go 1.26 以上，本版成品使用 Go 1.27.1。
- Windows：`./build_windows.ps1 -RequireInstaller`（需 Inno Setup）；Linux：`bash ./build_linux.sh`。測試：`go test ./...`、`go vet ./...`，以及 `scripts/test_*.cjs`（Node.js）。
- v0.3.13 新增區網 mDNS 探索公告，詳見 [LAN_DISCOVERY](docs/LAN_DISCOVERY.md)。
- [驗證與限制](VALIDATION.md)：Windows 及 Linux amd64 測試已通過；ARM64 實機及安裝升級流程未重測。

## 授權

Copyright (c) 2026 SpeedBackup contributors.

本專案自有程式碼採用 **GNU General Public License v3.0 only（SPDX: GPL-3.0-only）**，完整條款見 [LICENSE](LICENSE)，授權文字與 [YAWAsau/backup_script](https://github.com/YAWAsau/backup_script/blob/master/LICENSE) 相同。

程式不提供任何擔保；使用、修改及散布須遵守 GPL-3.0。第三方依賴保留其原有授權及著作權聲明，詳見 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt) 與 vendor 內各依賴的 LICENSE。

`v0.3.13-webdav14-gpl3` 為授權更新版，功能與程式版本仍為 v0.3.13-webdav14；同一 Release 提供完整對應原始碼。先前 MIT 發布版本與既有授權保留於歷史紀錄。

## 終端即時監看（watch）

已自啟動的服務直接用 `watch` 連接，不要再啟動第二份 `serve`：

```powershell
& "C:\Program Files\SpeedBackup Server\speedbackup-server.exe" watch
```

輸入管理網頁的管理員帳號及密碼（密碼不顯示，不是 WebDAV 分享帳號）。預設連接 `http://127.0.0.1:8765`，每秒刷新上傳／下載速度、本次服務運行累計、每個檔案的進度與 ETA、近期完成／失敗及移動／刪除等操作。未知大小顯示「大小未知」；速度由兩次實際取樣計算，暫停傳輸會歸零；尚未完成的請求最多顯示 99.9%。這是伺服器傳輸進度，不代表手機解壓／安裝完成。

關閉視窗或 Ctrl+C 只退出監看，不停止服務。連線失敗會清除速度樣本並重試；登入過期或服務重啟造成登入失效時，重新執行命令登入。畫面會依終端尺寸裁切，請放大視窗以看更多項目。TTY 使用原地刷新，重導向輸出自動改為追加快照。

可攜版解壓後執行 `.\speedbackup-server.exe watch`，即可監看已安裝的 webdav11 服務，**無需先安裝新版或重啟服務**。Linux：`./speedbackup-server watch`。可選 `--url https://server.example:8765`、`--username admin`、`--interval 500ms`、`--plain`、`--once`；遠端必須 HTTPS 或 SSH localhost tunnel。可用 `--token-file <既有有效 API Token 檔>` 取代帳密登入，或 `--username admin --password-stdin` 從標準輸入讀取密碼。不要把密碼直接寫進命令列／歷史紀錄。監看不建立設定、不重設 Token、不保存密碼。

Windows / Linux 共用的 SpeedBackup 專用 WebDAV 伺服器。本版加入設定自動套用、Windows 原生資料夾選擇、管理頁開機自啟控制、深黑與白色主題，並修正手機導覽與傳輸卡片排版。直接使用 IP 與埠號的根網址即可連接 WebDAV；帳號登入、匿名分享、目錄選擇、並行傳輸與開機啟動選項均保留。Android 腳本與 Dex 不需修改。

## 不開網頁與終端查看

分享設定完成後，WebDAV 服務獨立運行，關閉瀏覽器不影響備份／恢復。已安裝為服務者保持服務運行即可，不要再對同一資料目錄、埠號啟動第二份 `serve`。可攜版以前景啟動（關閉這個終端會結束前景服務）：

```powershell
.\speedbackup-server.exe serve --root "D:\SpeedBackupServerData" --listen 0.0.0.0:8765
```

提供只讀紀錄查看命令，可在**另一個終端**追蹤正在運行的服務，不必開網頁。`--root` 必須填原服務的資料目錄（管理頁「設定」的備份根目錄），不是 `F:\test` 等分享目錄：

```powershell
.\speedbackup-server.exe logs --root "D:\SpeedBackupServerData" --follow --lines 20
```

Linux 套件預設資料目錄的對應命令：

```sh
sudo -u speedbackup speedbackup-server logs --root /var/lib/speedbackup-server/data --follow
```

Windows 服務資料可能需要以系統管理員開啟終端才能讀取。使用自訂資料目錄時替換 `--root`。`logs` 只讀現有事件，Ctrl+C 只退出查看；不重啟或停止服務。每筆完成／失敗的 PUT、GET 顯示時間、檔案、帳號、HTTP 狀態、bytes、耗時及錯誤；不是逐請求 HEAD／PROPFIND／MOVE 的除錯追蹤，也不把手機解壓／安裝當成已完成。即時累計 bytes 仍可由管理頁查看。首次建立分享仍使用管理頁。

## 即時狀態與統計範圍

開始／完成事件立即喚醒監看，傳輸中每 50 ms 取樣；前端合併突發更新，最多每秒 20 次。閒置使用 25 秒長輪詢心跳，隱藏頁面取消監看。滾動時統計持續更新，清單只建立可見列及少量預先保留列。

即時壓縮的 PUT 通常沒有預先提供 Content-Length：進行中顯示已傳輸量與「總大小尚未確定」，成功後顯示實際大小；不推算未知總長的百分比。速度欄為該檔案的平均傳輸速度。檔案之間真正沒有 GET／PUT 時，傳輸中顯示 0，並保留累計完成數。

上傳／下載 bytes、完成數、異常數以**本次服務運行**累計，不因 200 筆近期清單輪替而下降，服務重啟才歸零；傳輸量包含失敗前已傳輸 bytes，並不代表磁碟佔用或唯一備份檔大小。「分享與備份」列出分享設定與近期傳輸路徑中的備份名稱，非磁碟完整索引。WebDAV 不提交裝置 ID／Profile／Generation；另一套 API 的 Manifest、CAS 儲存與工作階段功能仍保留。

## 首次登入與升級

在伺服器本機開啟 `http://127.0.0.1:8765/web/admin`，首次顯示「建立管理員」，填入帳號、非空白密碼、確認密碼。帳號使用1–48位小寫英文字母、數字、底線或短橫線，首字必須是英文字母或數字。之後直接以帳號密碼登入，不必保存登入Token。登入狀態8小時有效；重啟伺服器或重設密碼會要求重新登入。

從webdav1／go2升級：保留原有備份帳號、資料與API設定；因舊版沒有管理員帳號，第一次在本機建立即可，不需要找回舊Token。已有管理員的安裝不會再次提供註冊，也不會因升級重設密碼。管理員與手機WebDAV備份帳號分開。

首次建立管理員僅接受localhost，避免尚未設定的升級站點被區網訪客搶先接管。一般登入可從區網進行。無桌面的Linux可先建立SSH轉送：
```sh
ssh -L 18765:127.0.0.1:8765 your-user@server-ip
```
接著在自己的瀏覽器開啟 `http://127.0.0.1:18765/web/admin` 建立管理員。也可使用下方本機重設命令建立。初始設定完成後再配置反向代理。HTTP只供可信區網使用；外部連線使用HTTPS。

## 分享目錄：輸入或瀏覽

在「備份與恢復」選擇備份帳號，「分享目錄」可直接輸入伺服器的絕對路徑，或按「瀏覽…」，進入目錄後按「使用此目錄」，最後按「儲存帳號」。選擇器顯示**伺服器**磁碟／目錄，不是瀏覽器所在電腦的資料夾，Windows服務與無桌面的Linux都能使用。

- Windows例：`D:\PhoneBackups`；Linux例：`/srv/speedbackup/phone`。
- 自訂目錄必須已存在，服務帳號需要讀取、列舉、寫入及刪除暫存檔的權限。沒有權限會拒絕儲存並提示，不會偷偷改成其他目錄。
- 留空使用 `<ServerRoot>/webdav/<備份帳號>/`，舊帳號維持原目錄。
- 分享目錄不會自動加上帳號子目錄；選擇的路徑直接對應該帳號的 `/dav/`。
- 設定變更**不搬移、不刪除原備份**，只影響新請求。請在該帳號沒有進行中的備份／恢復時切換。
- 拒絕分享Server內部設定目錄或其上層，以及其他備份帳號的重疊目錄。Windows先列磁碟名稱，避免為列出磁碟而碰觸離線網路磁碟；選取離線目錄時會顯示讀取逾時。
- Windows服務看見的是其執行帳號可見的磁碟；互動登入者的網路磁碟映射不一定對LocalSystem可見。可直接輸入服务帳號可存取的UNC路徑。Linux服務預設帳號為speedbackup；請將選定備份目錄授予該帳號所需權限，不要把全系統目錄設成任何人可寫。

例如原rclone共享根目錄內有 `Backup_zstd_0/`，將其所在的父目錄選為分享目錄，保留原有備份層級即可。不需要搬到新Server預設目錄。

## 匿名分享

管理頁勾選「匿名分享」並儲存，會開放此分享目錄的無帳密 WebDAV 讀寫，只有一個啟用中的匿名分享時，直接使用 `http://IP:埠號/`；多個匿名分享使用各自的 `/dav-public/分享名稱/`，不任選根目錄。其他分享與管理介面不會變成匿名。腳本使用頁面產生的地址，`webdav_remote_user=''`、`webdav_remote_pass=''`；不需要修改 Dex 或腳本協議。分享名稱仍必填，用來選定目錄。

既有帳密模式預設不變。啟用匿名時保留原帳密，原 `/dav/` 客戶端仍可使用；關閉匿名或停用帳號立即拒絕新匿名請求（已開始的串流可能仍完成）。只有匿名、從未設過密碼的分享，改回帳密模式時須設定密碼。知道匿名網址且能連上服務的人可讀寫該目錄。

## 手機連接

管理頁會產生以下腳本設定，可一鍵複製（含本次輸入的密碼）。先儲存備份帳號，再取代腳本同名欄位；既有密碼無法反查，留空不會產生假的密碼設定。引號、美元符號等字元會依 shell 語法處理。

地址依實際 WebDAV 監聽 IP／埠與目前連線的伺服器 IPv4 產生。指定 IP 監聽或只有一個可用 IPv4 時自動帶入；0.0.0.0 多網卡且由 localhost 登入時不猜測手機路由，需選擇同網路地址，或從區網地址登入自動確認。系統預設路由僅排列候選項，不當成已確認地址；服務僅監聽 localhost 時會提示手機無法連接。自訂網域地址可手動輸入並保留。

可直接使用 `http://IP:埠號/`。帳密模式依帳號選目錄；匿名模式在只有一個啟用中的匿名分享時直接使用根網址，多個匿名分享則產生各自的完整路徑。有效帳密優先選定其帳號，錯誤帳密不降級成匿名。複製設定時也會核對，不需要先離開輸入框。

沿用原腳本欄位，IP與密碼改成自己的：
```sh
remote_type=webdav
webdav_url=http://192.168.0.100:8765/
webdav_remote_user=phone
webdav_remote_pass="自己設定的備份帳號密碼"
```
`remote_stream`保持原設定。使用管理頁產生的完整URL並保留末尾`/`；帳密或單一匿名分享可直接填根網址；舊 `/dav/` 與 `/dav-public/分享名稱/` 保留相容。不要填Server實體路徑或管理頁網址。手機不填電腦的127.0.0.1。換分享目錄時，WebDAV URL和帳號可保持不變。

根網址中的 `api`、`web`、`dav`、`dav-public` 第一層名稱由伺服器功能保留。如原始資料含同名目錄，可從原 `/dav/` 或各分享網址存取。管理頁仍為 `/web/admin`。WebDAV 傳輸直接處理，不靠 HTTP 跳轉；一般瀏覽器開啟根網址會導向管理頁。

## 忘記管理密碼

已登入時可在「設定」輸入目前密碼並更新新密碼。忘記密碼時，只在伺服器本機操作，停止服務後重設；不需要舊密碼或Token，也不會刪除備份。

Windows安裝版（系統管理員PowerShell；若安裝路徑或資料目錄不同請替換）：
```powershell
Set-Location 'C:\Program Files\SpeedBackup Server'
.\speedbackup-server.exe service stop
powershell -NoProfile -ExecutionPolicy Bypass -File .\reset_admin.ps1 -Root 'C:\ProgramData\SpeedBackup Server\data' -Username admin
.\speedbackup-server.exe service start
```
重設腳本以隱藏輸入讀取密碼，再經stdin傳入，不把密碼放在命令列參數或網址。可攜版先停止原serve程序，再用其原本`--root`執行重設腳本，完成後重新啟動。

Linux安裝版（Bash，使用服務帳號寫入設定，避免root擁有的新檔造成讀取失敗）：
```bash
sudo systemctl stop speedbackup-server
read -r -s -p 'New admin password: ' new_admin_password; printf '\n'
printf '%s\n' "$new_admin_password" | sudo -u speedbackup /usr/bin/speedbackup-server admin-reset --root /var/lib/speedbackup-server/data --username admin --password-stdin
unset new_admin_password
sudo systemctl start speedbackup-server
```
可攜版使用原本啟動Server的使用者及資料根目錄執行相同`admin-reset`命令。服務仍在執行時重設會被根目錄鎖拒絕。重設管理員不會重設手機的備份帳號密碼。

## 保留的功能及進度界線

標準WebDAV核心為golang.org/x/net/webdav；原Protocol v1 API、CAS、Manifest、Sessions、事件、清理與管理功能均保留。舊API Token仍供既有API客戶端使用；管理頁已不再要求Token，也不將Session放進JavaScript儲存區。API Token的輪替仍可透過原API完成。

PUT先写入暫存檔再發布，維持手機.part→HEAD→MOVE流程，支援Range下載及即時目錄列舉。成功MOVE後不查舊來源；省略Overwrite時依規範使用T。WebDAV資料不納入CAS統計或孤立物件清理。

頁面顯示檔案傳輸、bytes、單檔／Range百分比、速度、耗時與中斷。chunked未提供長度時不捏造百分比。**傳輸完成不代表手機解壓、安裝或權限恢復完成。** 即時列表保留最近200筆及所有進行中傳輸，可滾動查看；畫面只繪製可見列，完成事件保留於日誌。

## 傳輸列表與瀏覽效能

「備份與恢復」及「傳輸工作階段」都顯示 WebDAV 上傳／下載紀錄。既有 API 上傳工作仍保留，有資料時顯示於下方。即時列表只保留本次伺服器啟動後的近期紀錄；重啟後可至事件日誌查閱已完成的傳輸。

開始／完成事件立即喚醒、進度每 50 ms 取樣，前端突發更新最多每秒10次，閒置長輪詢25秒。虛擬清單重用可見列，滾動不暫停統計；閱讀歷史時保留所在列，隱藏或離頁即取消監看。沒有觀看頁面時不建立背景進度取樣。事件日誌延後繪製螢幕外的內容。

## 並行傳輸與資源使用

同一帳號的不同檔案可同時串流上傳，無關檔案的 MOVE 不必等慢速上傳結束。同檔案、來源／目的地、父子目錄操作會協調至暫存檔發布完成，避免 PUT 與父目錄改名相撞；排隊請求可以取消，標準 WebDAV LOCK token 仍由 x/net 處理。保留成功回應前的檔案 Sync、暫存檔發布與失敗清理，完成事件日誌也維持同步寫入。

進度採每條傳輸獨立的原子計數，讀寫資料塊不再取得全域進度 mutex。目錄以批次列舉，常見備份格式的 MIME 類型直接判定，不逐檔開啟內容嗅探；未知格式保留原偵測，檔案列表不使用過期快取。Windows 路徑別名透過系統完整路徑查詢解析，Linux 使用標準路徑解析，兩者執行同一套操作協調與資料保護。

前版 webdav6 的並行協調與資料保護仍保留。本版以 32 路、128 組有意限速的混合傳輸驗證即時畫面及內容 SHA，並非最大吞吐測試。更多同時傳輸可能增加 CPU、記憶體及磁碟壓力；事件通知不等於傳輸必然更省 CPU。效能數據為本機 loopback，不能當成手機 Wi-Fi／實體網卡速度或長時間穩定性保證。詳見 `VALIDATION_WEBDAV.md`。

## 安裝與建置

Windows：執行對應 Setup。首次安裝顯示可編輯的資料目錄；偵測到既有安裝後沿用原位置並跳過此頁，不搬移資料。連線頁提供「開機自動啟動」勾選；升級預設讀取既有服務啟動方式，可自行更改。安裝後會啟動本次服務，勾選只決定下次開機的行為。靜默安裝可用 `/AUTOSTART=true` 或 `/AUTOSTART=false`。完成頁直接開啟管理介面建立帳號。Linux：提供amd64／arm64可攜包及deb；例如 `sudo apt install ./speedbackup-server_0.3.10+webdav11-1_amd64.deb`。兩個平台共用帳號驗證、分享目錄驗證及 WebDAV 邏輯；磁碟列舉與路徑別名解析採對應平台實作。

Windows Setup 的區網防火牆選項只放行安裝後的程式與指定 TCP 埠號，來源限制為同一區網（LocalSubnet），適用公用、私人及網域網路。升級時勾選此選項會更新舊規則。Linux 不自動改動主機防火牆；若主機啟用防火牆，需按使用的發行版放行相應 TCP 埠號。

可攜版：`speedbackup-server serve --root <資料目錄> --listen 0.0.0.0:8765`。此資料目錄保存Server設定與預設備份目錄，與每個帳號可選的分享目錄不同。升級及卸載仍保留備份資料。

建置需要Go1.26以上；本版使用Go1.27.1、CGO_ENABLED=0，附vendor，可離線建置。執行`go test ./...`、`go vet ./...`；Windows套件用`build_windows.ps1 -RequireInstaller`，Linux用`packaging/linux/build_packages.sh`。

授權見LICENSE／THIRD_PARTY_NOTICES.txt。此版測試與尚未驗證項目見VALIDATION_WEBDAV.md；歷史驗證見各舊版交付，不能當成本版驗證。

## 開機啟動選項

Windows 安裝精靈與可攜包 `install_service_admin.ps1` 均可選擇。之後在管理員終端執行：
```powershell
.\speedbackup-server.exe service autostart --enabled=true
.\speedbackup-server.exe service autostart --enabled=false
```
Linux deb 可用隨附的安裝入口選擇；互動時會詢問，非互動時指定參數：
```sh
sudo sh install_linux.sh --autostart off ./speedbackup-server_0.3.10+webdav11-1_amd64.deb
sudo sh install_linux.sh --autostart on ./speedbackup-server_0.3.10+webdav11-1_amd64.deb
```
`keep` 沿用目前設定，全新安裝預設開啟。直接使用 apt/dpkg 仍沿用原套件預設。已安裝後也可用 `sudo speedbackup-server service autostart --enabled=true` 或 `false`；此命令只切換開機策略，不立即啟動／停止服務。Linux 使用 systemd，需先安裝 deb 或自行建立提供的 service unit；可攜程式不會自行註冊背景服務。

## v0.3.10：事件日誌顯示修復

- 修正已有 WebDAV 操作紀錄時，事件頁顯示 `say is not defined`：用途分類引用的語言函式未在事件頁定義。此為管理頁渲染錯誤，並非該訊息所指的檔案傳輸失敗。
- 加入實際事件 renderer 的繁簡中文、混合事件、毫秒時間、舊紀錄、安全文字輸出與切頁回歸；瀏覽器驗證會檢查有資料的事件列及頁面錯誤框，不能僅以無未捕获例外判定通過。
- Windows/Linux 共用同一修復；WebDAV 協定與備份格式保持相容。請在傳輸結束後更新安裝版，避免安裝程式重啟服務影響正在進行的操作。

## 日誌與除錯包

管理頁「設定 → 日誌與除錯」及「事件日誌」提供「下載伺服器除錯包」。會直接下載 ZIP 到瀏覽器的下載位置；可在傳輸中使用，不必重啟服務。網頁本身發生渲染錯誤時，錯誤畫面也保留下載連結。

- `server-info.json`：版本、作業系統、Go版本、啟動時間／運行時間、監聽地址、日誌位置、正在進行及近期傳輸、日誌寫入錯誤與遺漏計數。
- `logs/server.jsonl*`：所有 HTTP/WebDAV 請求的方法、路徑、帳號、來源 IP、目的路徑、狀態碼、上下行位元組、耗時、底層錯誤與請求編號；含 HEAD／PROPFIND。網頁錯誤記錄頁面、錯誤名稱／訊息和程式行號。
- `audit/events.jsonl*`：操作完成／失敗與服務啟停；WebDAV 操作有 request_id 可對照詳細請求。
- `bundle-manifest.json`：匯出檔案、裁切、缺失與不完整行提示。日誌是滾動保留，不是永久完整歷史；異常退出可能遺失尚未批次寫出的約200毫秒紀錄。隊列達2MiB時不阻塞傳輸，遺漏筆數會顯示在除錯包與設定頁。

詳細日誌位於「服務資料目錄/.speedbackup-server/logs/」，操作日誌位於「服務資料目錄/.speedbackup-server/audit/」；管理頁顯示實際完整路徑。各保留目前及3份輪替檔，每份約8MiB，新的日誌合計約64MiB；旧超大檔逐步輪替淘汰，除錯包每檔最多取末8MiB。事件頁只讀最近需要的尾端紀錄。

除錯包只讀指定的日誌檔案，不打包設定／帳密檔、Token、Cookie、Authorization、請求或回應內容、備份內容、環境變數。日誌包含檔名／路徑、帳號名稱與 IP。ZIP 直接下載，不在伺服器累積除錯包副本。

終端也可匯出（指定的是服務資料目錄，而非WebDAV分享目錄；需要讀取日誌的作業系統權限）：

```text
speedbackup-server debug-export --root "服務資料目錄" --output "speedbackup_server_debug.zip"
```

已存在的輸出檔不會覆蓋。終端使用當下檔案快照，網頁下載另外包含活動傳輸與記錄器狀態；運行中輪替可能略過個別檔，清單會標示讀取錯誤。新詳細日誌功能需安裝新版後才開始記錄，無法補回舊版未曾記錄的內容。

## v0.3.9：操作用途與精確時間

- WebDAV 操作紀錄包含 GET／PUT／DELETE／MOVE／COPY／MKCOL；保留 HTTP 結果、來源與目的路徑。查詢 HEAD／PROPFIND／OPTIONS 不放進主要操作清單，避免查詢洗掉操作歷史。
- 依操作與檔名顯示「備份數據上傳」「恢復數據下載」；JSON 與 app_details 資訊封裝顯示「更新 JSON 列表」「獲取應用資訊」。這是用途推定，伺服器不解析檔案內容或推測手機內部進度。
- `.part.<時間>.<序號>` 移到同名正式檔案顯示「提交備份數據／JSON 列表」；其他 MOVE 顯示移動／重新命名，並保存完整來源、目的路徑。失敗操作保留失敗狀態；操作不重複計算上下載流量。
- 清單與事件日誌顯示伺服器開始時間，含日期、毫秒及瀏覽器本地時區。終端日誌使用伺服器本地時區並包含毫秒；這是記錄解析度，實際準確度取決於伺服器時鐘。移動／複製的完整路徑可在事件日誌查看。
- 保留最近 200 筆已結束操作及所有進行中操作；累計上下載位元組僅來自 PUT／GET，重啟時歸零。手機版加入獨立時間列，仍以虛擬清單局部更新。

## v0.3.8：原生選擇器修正與表單排版

- 修正 Windows 正規化啟動網址時補上 `/`，造成原生資料夾選擇器顯示 `invalid folder-picker request`。新網址明確使用根路徑，解析器接受新舊格式；其他路徑、額外參數與任意回呼仍不接受。
- 管理內容改為無外框、無陰影的區塊。已建立分享置於表單上方；桌面採左標籤／右欄位，手機採上下排列。輸入欄位、按鈕與對話視窗保留辨識邊界。
- 參考 SFTPGo 管理頁的導覽與表單組織方式，使用本專案原有 HTML/CSS/JS 自行實作，未複製其 KeenThemes UI 程式或資產。傳輸仍採虛擬清單與局部更新，無新增前端框架。

## 日常管理

- 分享欄位填好後，離開欄位或按 Enter 自動套用；勾選匿名／停用立即套用。畫面顯示正在套用、已套用或錯誤；失敗時保留原設定。既有分享名稱固定，使用「新增分享」建立另一個分享。密碼只留在目前頁面，不存入瀏覽器儲存空間。
- 設定提供跟隨系統、深色、深黑與白色，記住此瀏覽器的選擇。移除 WebDAV 腳本不使用的 Manifest／CAS 儲存空間頁；既有後端 API 與資料保留。
- 設定顯示作業系統開機自啟狀態，可切換開／關；不會停止正在運行的服務。Windows 安裝服務及 Linux systemd 套件支援；手動執行的可攜程式不能更改另一個已安裝服務的開機設定。
- Windows 安裝版在伺服器本機 `http://127.0.0.1:連接埠/web/admin` 按「瀏覽…」，會開啟 Windows 原生資料夾選擇器。瀏覽器可能詢問是否允許開啟 SpeedBackup。若未開啟可按「改用網頁瀏覽」。手機、遠端瀏覽器、HTTPS、Linux 與可攜模式使用伺服器目錄瀏覽；也可直接輸入路徑。選定後由服務再次驗證路徑及存取權限。
- 原生選擇器使用安裝的 `speedbackup-picker.exe` 與專用 URL 協定；一次性要求限本機、有效 3 分鐘，結果只交回原管理員工作階段。小程式核對埠所屬的本機 Windows 服務，不接受任意回呼位址，也不取得管理員密碼。
- Linux 自啟控制使用 systemd 啟動的本機 socket helper；主服務維持 speedbackup 非 root 帳號。helper 只接受開／關自己的單一服務，檢查 Unix peer 的 UID 與正在運行的服務 PID，不接受任意命令。
- 手機管理網址是 `http://電腦區網IPv4:連接埠/web/admin`；設定內提供可複製網址。手機上的 127.0.0.1 指手機自己。先在伺服器本機建立管理員，之後區網可用管理員帳密登入；匿名分享不代表匿名管理權限。

手機版使用緊湊的兩欄導覽、獨立一列的重新整理／語言控制、縱向分享欄位及傳輸卡片。傳輸卡片仍使用虛擬清單，捲動時持續刷新統計；桌面保留表格排列。

## webdav13：外觀與連線可靠性

- 設定提供四種主題預覽、藍／青綠／紫強調色、桌面緊湊排列與減少動畫；立即套用並保存於目前瀏覽器，可還原預設。既有主題自動保留。手機維持觸控間距。
- 管理頁一般請求及回應內文讀取有 15 秒期限；傳輸長輪詢為 35 秒。寫入逾時表示結果尚未確認，請重新讀取狀態；不會自動重送設定、密碼或其他寫入。
- watch 閒置時利用服務端 revision 長輪詢（最長 25 秒），有事件立即喚醒；傳輸時保留 --interval 設定。Ctrl+C 可立即取消等待，缺少 revision 的舊服務保留定時查詢。
- 參考 PatternFly 的表單分組、Nextcloud 的語意色彩變數、File Browser 的個人偏好分組，以現有原生 JS/CSS 實作，未引入額外前端框架。SFTPGo 的全域連線限制需要獨立容量驗證，本版未直接套用，避免影響既有多檔傳輸。

設計參考：https://www.patternfly.org/components/forms/form/html/ 、https://docs.nextcloud.com/server/stable/developer_manual/html_css_design/css.html 、https://github.com/filebrowser/filebrowser/blob/master/frontend/src/views/settings/Profile.vue 、https://github.com/sftpgo/docs/blob/main/docs/config-file.md

## webdav15：單一儀表板與流暢監看

登入後預設開啟儀表板；即時傳輸只保留一處。「分享與備份 → 管理分享」保留帳號、目錄及連線設定。移除重複的備份與恢復、傳輸工作階段側欄項目。

傳輸中取樣 50ms，進度條平滑過渡至已收到的數值，不推測未傳輸的 bytes。開啟減少動畫時不使用過渡。只有進度變動時傳送 active delta；開始／完成／失敗、重連與首次讀取傳回完整最近 200 筆紀錄。舊 API 呼叫保留完整回應；閒置維持 25 秒長輪詢，背景頁面停止監看。
