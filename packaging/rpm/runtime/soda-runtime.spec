Name:           soda-runtime
Version:        0.4.0
Release:        1%{?dist}
Summary:        Soda OS host composition
License:        MIT OR Apache-2.0
Requires:       ca-certificates, iproute, nftables-services, openssh-server, policycoreutils, shadow-utils, soda-forgejo = 15.0.7, systemd, tailscale, util-linux-core

%description
Tailnet enrollment, OpenSSH, firewall, console guidance, and upstream service
composition for the Soda OS appliance.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_prefix}/lib/soda/network %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_unitdir}/getty@tty1.service.d %{buildroot}%{_unitdir}/nftables.service.d %{buildroot}%{_presetdir} %{buildroot}%{_sysctldir} %{buildroot}%{_sysconfdir}/profile.d
install -m 0755 %{_sourcedir}/soda-tailnet %{buildroot}%{_bindir}/soda-tailnet
install -m 0755 %{_sourcedir}/soda-console-welcome %{buildroot}%{_libexecdir}/soda/soda-console-welcome
install -m 0644 %{_sourcedir}/soda-tailscale-enroll.service %{buildroot}%{_unitdir}/soda-tailscale-enroll.service
install -m 0644 %{_sourcedir}/90-soda.preset %{buildroot}%{_presetdir}/90-soda.preset
install -m 0644 %{_sourcedir}/soda-ingress.nft %{buildroot}%{_prefix}/lib/soda/network/soda-ingress.nft
install -m 0644 %{_sourcedir}/10-soda-ingress.conf %{buildroot}%{_unitdir}/nftables.service.d/10-soda-ingress.conf
install -m 0644 %{_sourcedir}/10-soda-console.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
install -m 0644 %{_sourcedir}/60-soda-console.conf %{buildroot}%{_sysctldir}/60-soda-console.conf
install -m 0644 %{_sourcedir}/soda-console-welcome.sh %{buildroot}%{_sysconfdir}/profile.d/soda-console-welcome.sh

%files
%{_bindir}/soda-tailnet
%{_libexecdir}/soda/soda-console-welcome
%{_unitdir}/soda-tailscale-enroll.service
%{_presetdir}/90-soda.preset
%{_prefix}/lib/soda/network/soda-ingress.nft
%{_unitdir}/nftables.service.d/10-soda-ingress.conf
%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
%{_sysctldir}/60-soda-console.conf
%{_sysconfdir}/profile.d/soda-console-welcome.sh
