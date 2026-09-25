# v0.3.13-webdav14 validation

Original release validation: Windows Go package tests and go vet passed, including server LAN announcement tests. mDNS was verified on a local network with an independent client. These results apply to that test environment, not every router or network.

Windows x64 installer and portable binaries, and Linux amd64/arm64 portable binaries are provided. Linux binaries were cross-compiled; this version has not been runtime-tested on Linux/ARM64. Windows/Debian install-upgrade lifecycle was not rerun. No Debian package is published for this version.

The public package removes private diagnostic reports and Android build metadata. Product Go code and embedded WebAdmin assets are unchanged from the archived webdav14 source. The README and CI are updated for standalone publication. Public source rebuilding is checked against the executable bytes before publication.

External HTTPS hosting is not configured by installing this release. Local HTTP should be used only on trusted networks; public access requires a separately configured encrypted endpoint.
