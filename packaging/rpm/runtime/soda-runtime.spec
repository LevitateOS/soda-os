Name:           soda-runtime
Version:        %{soda_version}
Release:        1%{?dist}
Summary:        Soda OS host composition
License:        MIT OR Apache-2.0
Requires:       ca-certificates, firewalld, iproute, NetworkManager, openssh-server, policycoreutils, shadow-utils, soda-forgejo = 15.0.7, systemd, tailscale, util-linux-core

%description
Tailnet enrollment, OpenSSH, firewall, console guidance, and upstream service
composition for the Soda OS appliance.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_unitdir}/getty@tty1.service.d %{buildroot}%{_presetdir} %{buildroot}%{_sysctldir} %{buildroot}%{_sysconfdir}/firewalld/zones %{buildroot}%{_sysconfdir}/profile.d
install -m 0755 %{_sourcedir}/soda-tailnet %{buildroot}%{_bindir}/soda-tailnet
install -m 0755 %{_sourcedir}/soda-local-access %{buildroot}%{_bindir}/soda-local-access
install -m 0755 %{_sourcedir}/soda-console-welcome %{buildroot}%{_libexecdir}/soda/soda-console-welcome
install -m 0644 %{_sourcedir}/90-soda.preset %{buildroot}%{_presetdir}/90-soda.preset
install -m 0644 %{_sourcedir}/soda-tailnet.xml %{buildroot}%{_sysconfdir}/firewalld/zones/soda-tailnet.xml
install -m 0644 %{_sourcedir}/10-soda-console.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
install -m 0644 %{_sourcedir}/60-soda-console.conf %{buildroot}%{_sysctldir}/60-soda-console.conf
install -m 0644 %{_sourcedir}/soda-console-welcome.sh %{buildroot}%{_sysconfdir}/profile.d/soda-console-welcome.sh

%files
%{_bindir}/soda-tailnet
%{_bindir}/soda-local-access
%{_libexecdir}/soda/soda-console-welcome
%{_presetdir}/90-soda.preset
%config(noreplace) %{_sysconfdir}/firewalld/zones/soda-tailnet.xml
%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
%{_sysctldir}/60-soda-console.conf
%{_sysconfdir}/profile.d/soda-console-welcome.sh
