Name:           soda-runtime
Version:        %{soda_version}
Release:        1%{?dist}
Summary:        Soda OS host composition
License:        MIT OR Apache-2.0
Requires:       cockpit-system, cockpit-networkmanager, cloud-init, ca-certificates, coreutils, firewalld, glibc-common, iproute, NetworkManager, openssh-server, policycoreutils, shadow-utils, sudo, soda-forgejo = 15.0.7, systemd, tailscale, util-linux-core

%description
Tailnet enrollment, OpenSSH, firewall, console guidance, and upstream service
composition for the Soda OS appliance.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_unitdir}/getty@tty1.service.d %{buildroot}%{_presetdir} %{buildroot}%{_sysctldir} %{buildroot}%{_tmpfilesdir} %{buildroot}%{_sysconfdir}/profile.d
install -m 0755 %{_sourcedir}/soda-tailnet %{buildroot}%{_bindir}/soda-tailnet
install -m 0755 %{_sourcedir}/soda-console-welcome %{buildroot}%{_libexecdir}/soda/soda-console-welcome
install -m 0644 %{_sourcedir}/90-soda.preset %{buildroot}%{_presetdir}/90-soda.preset
install -m 0644 %{_sourcedir}/soda-runtime.tmpfiles %{buildroot}%{_tmpfilesdir}/soda-runtime.conf
install -m 0644 %{_sourcedir}/10-soda-console.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
install -m 0644 %{_sourcedir}/60-soda-console.conf %{buildroot}%{_sysctldir}/60-soda-console.conf
install -m 0644 %{_sourcedir}/soda-console-welcome.sh %{buildroot}%{_sysconfdir}/profile.d/soda-console-welcome.sh

mkdir -p %{buildroot}%{_datadir}/cockpit/soda-tailscale
install -m 0644 %{_sourcedir}/soda-tailscale-manifest.json %{buildroot}%{_datadir}/cockpit/soda-tailscale/manifest.json
install -m 0644 %{_sourcedir}/soda-tailscale-index.html %{buildroot}%{_datadir}/cockpit/soda-tailscale/index.html
install -m 0644 %{_sourcedir}/soda-tailscale-app.css %{buildroot}%{_datadir}/cockpit/soda-tailscale/app.css
install -m 0644 %{_sourcedir}/soda-tailscale-app.mjs %{buildroot}%{_datadir}/cockpit/soda-tailscale/app.mjs
install -m 0644 %{_sourcedir}/soda-tailscale-native.mjs %{buildroot}%{_datadir}/cockpit/soda-tailscale/native.mjs
install -m 0644 %{_sourcedir}/soda-tailscale-status.mjs %{buildroot}%{_datadir}/cockpit/soda-tailscale/status.mjs
install -m 0644 %{_sourcedir}/soda-tailscale-stream.mjs %{buildroot}%{_datadir}/cockpit/soda-tailscale/stream.mjs
install -m 0644 %{_sourcedir}/60-soda-tailscale.conf %{buildroot}%{_sysctldir}/60-soda-tailscale.conf

%files
%{_bindir}/soda-tailnet
%{_libexecdir}/soda/soda-console-welcome
%{_presetdir}/90-soda.preset
%{_tmpfilesdir}/soda-runtime.conf
%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
%{_sysctldir}/60-soda-console.conf
%config(noreplace) %{_sysconfdir}/profile.d/soda-console-welcome.sh

%{_datadir}/cockpit/soda-tailscale/
%{_sysctldir}/60-soda-tailscale.conf
