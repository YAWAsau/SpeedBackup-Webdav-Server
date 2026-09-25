Name:           speedbackup-server
Version:        0.3.10
Release:        1.webdav11%{?dist}
Summary:        SpeedBackup dedicated backup server and WebAdmin
License:        MIT AND BSD-3-Clause
Source0:        speedbackup-server
Source1:        speedbackup-server.service
Source2:        server.env
Source3:        LICENSE
Source4:        THIRD_PARTY_NOTICES.txt
Source5:        speedbackup-server-autostart.socket
Source6:        speedbackup-server-autostart@.service
BuildRequires:  systemd-rpm-macros
BuildArch:      %{_arch}
Requires(pre):  shadow-utils
Requires(post): systemd, util-linux
Requires(preun): systemd
Requires(postun): systemd

%description
SpeedBackup Server provides the dedicated SpeedBackup Protocol v1 backend,
transactional upload sessions, manifest generations, integrity verification,
and a browser WebAdmin.

%prep

%build

%install
install -D -m 0755 %{SOURCE0} %{buildroot}%{_bindir}/speedbackup-server
install -D -m 0644 %{SOURCE1} %{buildroot}%{_unitdir}/speedbackup-server.service
install -D -m 0640 %{SOURCE2} %{buildroot}%{_sysconfdir}/speedbackup-server/server.env
install -D -m 0644 %{SOURCE3} %{buildroot}%{_licensedir}/%{name}/LICENSE
install -D -m 0644 %{SOURCE4} %{buildroot}%{_licensedir}/%{name}/THIRD_PARTY_NOTICES.txt
install -D -m 0644 %{SOURCE5} %{buildroot}%{_unitdir}/speedbackup-server-autostart.socket
install -D -m 0644 %{SOURCE6} %{buildroot}%{_unitdir}/speedbackup-server-autostart@.service
install -d -m 0750 %{buildroot}%{_sharedstatedir}/speedbackup-server/data

%pre
getent group speedbackup >/dev/null || groupadd -r speedbackup
getent passwd speedbackup >/dev/null || useradd -r -g speedbackup -d /var/lib/speedbackup-server -s /sbin/nologin -c "SpeedBackup Server" speedbackup
exit 0

%post
set -e
%systemd_post speedbackup-server.service
install -d -o speedbackup -g speedbackup -m 0750 /var/lib/speedbackup-server /var/lib/speedbackup-server/data
CONFIG_DIR=/var/lib/speedbackup-server/data/.speedbackup-server/config
TOKEN_FILE=/var/lib/speedbackup-server/FIRST_RUN_TOKEN.txt
if [ ! -d "$CONFIG_DIR" ] || ! find "$CONFIG_DIR" -maxdepth 1 -type f -name '*.json' -print -quit 2>/dev/null | grep -q .; then
  runuser -u speedbackup -- /usr/bin/speedbackup-server init --root /var/lib/speedbackup-server/data --token-file "$TOKEN_FILE" >/dev/null
  chown root:root "$TOKEN_FILE"
  chmod 0600 "$TOKEN_FILE"
  echo "WebAdmin: create the administrator via localhost /web/admin. Legacy API token retained at $TOKEN_FILE."
fi
if [ -d /run/systemd/system ]; then
  if [ "$1" -eq 1 ]; then
    systemctl enable --now speedbackup-server.service
  fi
fi

%preun
%systemd_preun speedbackup-server.service

%postun
%systemd_postun_with_restart speedbackup-server.service

%files
%license %{_licensedir}/%{name}/LICENSE
%license %{_licensedir}/%{name}/THIRD_PARTY_NOTICES.txt
%{_bindir}/speedbackup-server
%{_unitdir}/speedbackup-server.service
%{_unitdir}/speedbackup-server-autostart.socket
%{_unitdir}/speedbackup-server-autostart@.service
%config(noreplace) %{_sysconfdir}/speedbackup-server/server.env
%dir %attr(0750,speedbackup,speedbackup) %{_sharedstatedir}/speedbackup-server
%dir %attr(0750,speedbackup,speedbackup) %{_sharedstatedir}/speedbackup-server/data

%changelog
* Mon Sep 14 2026 SpeedBackup <noreply@example.invalid> - 0.2.1-3.go3
- Add systemd deployment, first-run token provisioning, and WebAdmin service package.
