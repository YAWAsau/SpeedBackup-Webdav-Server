# SpeedBackup WebDAV 內網探測

Android：r718 discovery33，版本 202609240001。Server：0.3.13-webdav14。

在主選單選 **6 → 4**，掃描區網 WebDAV；**6 → 5** 可以指定網段、連接埠與服務路徑。結果顯示完整網址。掃描不會替你修改帳密或選定備份目的地。

先接收 `_webdav._tcp` / `_webdavs._tcp` 的 mDNS/DNS-SD 公告，再驗證實際 HTTP 回應；同時提供常用埠掃描。新版 SpeedBackup Server 公告自己的實際監聽埠，即使不是 8765 也能被找到。伺服器保留登入要求；根路徑 OPTIONS 回傳 DAV 能力，讓尚未登入的探測器能辨識協議。

| 結果 | 意義 |
|---|---|
| 已確認 WebDAV | 收到 DAV 類別標頭，或有效的 DAV 207 multistatus |
| 已確認 WebDAV，需要登入 | 協議已確認，但目前仍須認證 |
| 需要登入，尚未確認 WebDAV | 只有 HTTP 401/403，不足以證明是 WebDAV |
| HTTPS 憑證待確認 | 未通過憑證／主機名稱驗證；沒有跳過驗證 |

## 範圍與限制

- 自動掃描已連線的私有 IPv4 網段，每個介面最多其所在 /24。更大網段會在原始日誌顯示縮小範圍；進階可指定 /22 到 /32。
- 常用埠：80、443、8080、8081、8000、8443、8765、5005、5006、5244；常用路徑：`/`、`/dav/`、`/webdav/`。
- 單輪最多 8192 個 IP/埠組合、512 個 HTTP 驗證工作；TCP 96 路、HTTP 12 路；選單預算 30 秒。達限會保留已找到的結果並明確標成部分掃描。
- 只發出匿名 OPTIONS／PROPFIND Depth:0；不帶設定中的密碼，不上傳、不刪除、不遞迴列目錄、不跟隨重新導向。
- 無公告的自訂埠、未知深層路徑、需要指定網域的虛擬主機、IPv6 與隔離網段不保證能找到。可用進階掃描補上已知埠與路徑。
- mDNS 通常限同一網段；AP 隔離、封鎖多播或防火牆可影響公告。TCP 掃描仍獨立運作。新版 Windows 安裝程式的區網防火牆選項加入本程式的 UDP 5353、限制 LocalSubnet；Linux 需允許區網 UDP 5353。

## 套用

Android PATCH 解壓到原產品目錄、合併 `tools/`。包含配套 tools.sh、dex_check.sh、classes.dex、speednative、tar；設定與 App 清單不在更新包。請在備份／還原結束後套用。

Server 可使用新版 Windows 安裝程式升級，或使用 portable 包；Linux portable 包提供可執行檔與啟動腳本。公告功能會在啟動新版 Server 後生效。此開發任務使用獨立測試服務與備用機暫存，未重啟既有安裝服務。

## CLI

```sh
CLASSPATH=/data/backup_tools/classes.dex app_process /system/bin com.xayah.dex.WebDavDiscoveryUtil scanWebDav
CLASSPATH=/data/backup_tools/classes.dex app_process /system/bin com.xayah.dex.WebDavDiscoveryUtil scanWebDav 192.168.1.0/24 8765,5005 /,/dav/ 30000 96 1
CLASSPATH=/data/backup_tools/classes.dex app_process /system/bin com.xayah.dex.WebDavDiscoveryUtil probeWebDav http://192.168.1.20:8765/
```

stdout 是六欄 TSV：URL、state、evidence、HTTP status、source、elapsedMs；摘要在 stderr。回傳 0：有確認結果；1：没有確認結果（可能有待確認候選）；2：參數無效；3：掃描達限／部分完成。`probeWebDav` 僅接受無帳密的 literal IPv4 HTTP(S) URL。

FULL_SOURCE 是建置來源，不是可直接執行的 Android 產品。測試範圍與限制見 REPORT.md。
