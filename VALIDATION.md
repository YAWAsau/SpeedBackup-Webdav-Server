# v0.3.14-webdav15 validation

Windows Go tests / vet and JavaScript tests pass, including delta lifecycle, truthful totals, idle waits, hidden-page cancellation, retained history and full resynchronization. Windows/Linux builds use Go 1.27.1 and CGO_ENABLED=0.

Isolated local HTTP transfer: 24 MiB SHA256 matched, with 200 completed records. Headless Chrome verified the single dashboard, retained share editor, desktop/mobile layout, reduced motion and monitor teardown. Progress sample median was 52ms; average progress delta 583 bytes versus full history response 54,248 bytes. Unchanged visible history rows had zero text mutations; only 9 virtual rows existed in the desktop sample. These measurements describe this local fixture, not physical-phone FPS or Internet performance.

Linux amd64 console/server/desktop/service runtime tests passed in an isolated LOCAL QEMU snapshot with guest external networking disabled. ARM64 runtime and Windows/Debian install-upgrade lifecycle are not validated in this change. Production services are not restarted or modified.

GPL-3.0-only applies to project code; third-party dependency notices remain unchanged.


## v0.3.15-webdav16 theme compatibility
- User confirmed whitefix2 with Dark Reader. Static darkreader-lock is embedded before styles/scripts.
- Explicit white opts out of Chromium Auto Dark Mode. Pixel comparison fails before the fix and passes after it.
- Theme switching and persistence checked using an isolated Windows executable.
- Windows tests/vet and JavaScript regressions rerun for release; Windows/Linux amd64/arm64 binaries rebuilt.
- Linux runtime results above belong to webdav15; no new Linux runtime, ARM64 hardware or installer upgrade testing for this theme-only change.


## v0.3.16-webdav17 glass UI
- User accepted glass8; release uses the same UI with a stable version number.
- All Windows Go tests/vet and JavaScript regressions rerun; Windows/Linux amd64/arm64 rebuilt.
- Isolated built Windows server: 4 themes x 4 pages, aligned sidebar labels, regular font weights, appearance buttons and keyboard activation, persistence, and mobile theme switching. Chrome reported OPPO Sans 4.0 Regular as the actual rendered custom font.
- Bundled font bytes checked against the official ZIP; font HTTP MIME, cache policy and SHA256 verified.
- Unified dashboard transfer fixture: 24 MiB SHA256 passed, progress median 53ms, no unchanged-history DOM mutations. Measurements are headless Chrome, not physical-phone GPU performance.
- ARM64 hardware and installer upgrade are not tested in this release. Linux runtime verification is provided by GitHub Actions before delivery.
