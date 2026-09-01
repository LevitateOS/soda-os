Name:           soda-runtime
Version:        0.4.0
Release:        1%{?dist}
Summary:        Soda OS residual host runtime composition
License:        MIT OR Apache-2.0
Requires:       ca-certificates, gcc, gcc-c++, git-core, iproute, make, nftables-services, openssh-clients, openssh-server, policycoreutils, policycoreutils-python-utils, pkgconf-pkg-config, shadow-utils, soda-forgejo = 15.0.7, systemd, tailscale, tar, unzip, util-linux-core, xz

%description
Temporary host telemetry RPCs and their administrator health CLI. Projects do
not use this daemon.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_prefix}/lib/soda/network %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_unitdir}/getty@tty1.service.d %{buildroot}%{_unitdir}/nftables.service.d %{buildroot}%{_presetdir} %{buildroot}%{_tmpfilesdir} %{buildroot}%{_sysctldir} %{buildroot}%{_sysusersdir} %{buildroot}%{_sysconfdir}/profile.d
install -m 0755 %{_sourcedir}/sodad %{buildroot}%{_libexecdir}/soda/sodad
install -m 0755 %{_sourcedir}/sodactl %{buildroot}%{_bindir}/sodactl
install -m 0755 %{_sourcedir}/soda-tailnet %{buildroot}%{_bindir}/soda-tailnet
install -m 0755 %{_sourcedir}/soda-console-welcome %{buildroot}%{_libexecdir}/soda/soda-console-welcome
install -m 0644 %{_sourcedir}/sodad.service %{buildroot}%{_unitdir}/sodad.service
install -m 0644 %{_sourcedir}/soda-tailscale-enroll.service %{buildroot}%{_unitdir}/soda-tailscale-enroll.service
install -m 0644 %{_sourcedir}/soda-state-directories.service %{buildroot}%{_unitdir}/soda-state-directories.service
install -m 0644 %{_sourcedir}/opt-soda-toolchains.mount %{buildroot}%{_unitdir}/opt-soda-toolchains.mount
install -m 0644 %{_sourcedir}/90-soda.preset %{buildroot}%{_presetdir}/90-soda.preset
install -m 0644 %{_sourcedir}/soda-ingress.nft %{buildroot}%{_prefix}/lib/soda/network/soda-ingress.nft
install -m 0644 %{_sourcedir}/10-soda-ingress.conf %{buildroot}%{_unitdir}/nftables.service.d/10-soda-ingress.conf
install -m 0644 %{_sourcedir}/10-soda-console.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
install -m 0644 %{_sourcedir}/soda.conf %{buildroot}%{_tmpfilesdir}/soda.conf
install -m 0644 %{_sourcedir}/60-soda-console.conf %{buildroot}%{_sysctldir}/60-soda-console.conf
install -m 0644 %{_sourcedir}/soda.sysusers %{buildroot}%{_sysusersdir}/soda.conf
install -m 0644 %{_sourcedir}/soda-console-welcome.sh %{buildroot}%{_sysconfdir}/profile.d/soda-console-welcome.sh

%files
%{_libexecdir}/soda/sodad
%{_bindir}/sodactl
%{_bindir}/soda-tailnet
%{_libexecdir}/soda/soda-console-welcome
%{_unitdir}/sodad.service
%{_unitdir}/soda-tailscale-enroll.service
%{_unitdir}/soda-state-directories.service
%{_unitdir}/opt-soda-toolchains.mount
%{_presetdir}/90-soda.preset
%{_prefix}/lib/soda/network/soda-ingress.nft
%{_unitdir}/nftables.service.d/10-soda-ingress.conf
%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
%{_tmpfilesdir}/soda.conf
%{_sysctldir}/60-soda-console.conf
%{_sysusersdir}/soda.conf
%{_sysconfdir}/profile.d/soda-console-welcome.sh
