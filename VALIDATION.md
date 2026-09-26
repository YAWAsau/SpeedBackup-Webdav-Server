# v0.3.14-webdav15 validation

Windows Go tests / vet and JavaScript tests pass, including delta lifecycle, truthful totals, idle waits, hidden-page cancellation, retained history and full resynchronization. Windows/Linux builds use Go 1.27.1 and CGO_ENABLED=0.

Isolated local HTTP transfer: 24 MiB SHA256 matched, with 200 completed records. Headless Chrome verified the single dashboard, retained share editor, desktop/mobile layout, reduced motion and monitor teardown. Progress sample median was 52ms; average progress delta 583 bytes versus full history response 54,248 bytes. Unchanged visible history rows had zero text mutations; only 9 virtual rows existed in the desktop sample. These measurements describe this local fixture, not physical-phone FPS or Internet performance.

Linux amd64 console/server/desktop/service runtime tests passed in an isolated LOCAL QEMU snapshot with guest external networking disabled. ARM64 runtime and Windows/Debian install-upgrade lifecycle are not validated in this change. Production services are not restarted or modified.

GPL-3.0-only applies to project code; third-party dependency notices remain unchanged.
