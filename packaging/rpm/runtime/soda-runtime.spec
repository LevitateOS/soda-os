Name:           soda-runtime
Version:        0.4.0
Release:        1%{?dist}
Summary:        Soda OS project runtime
License:        MIT OR Apache-2.0
Requires:       ca-certificates, gcc, gcc-c++, git-core, iproute, make, openssh-clients, openssh-server, policycoreutils, policycoreutils-python-utils, pkgconf-pkg-config, shadow-utils, soda-forgejo = 15.0.7, sqlite-libs, systemd, tailscale, tar, unzip, util-linux-core, xz

%description
Privileged project daemon, administrator CLI, and SSH session gateway for Soda OS.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_unitdir}/getty@tty1.service.d %{buildroot}%{_unitdir}/tailscaled.service.d %{buildroot}%{_presetdir} %{buildroot}%{_tmpfilesdir} %{buildroot}%{_sysctldir} %{buildroot}%{_sysusersdir} %{buildroot}%{_sysconfdir}/profile.d %{buildroot}%{_sysconfdir}/ssh/sshd_config.d %{buildroot}%{_sysconfdir}/soda/authorized_keys
install -m 0755 %{_sourcedir}/sodad %{buildroot}%{_libexecdir}/soda/sodad
install -m 0755 %{_sourcedir}/soda-ssh %{buildroot}%{_libexecdir}/soda/soda-ssh
install -m 0755 %{_sourcedir}/sodactl %{buildroot}%{_bindir}/sodactl
install -m 0755 %{_sourcedir}/soda-tailnet %{buildroot}%{_bindir}/soda-tailnet
install -m 0755 %{_sourcedir}/soda-console-welcome %{buildroot}%{_libexecdir}/soda/soda-console-welcome
install -m 0644 %{_sourcedir}/sodad.service %{buildroot}%{_unitdir}/sodad.service
install -m 0644 %{_sourcedir}/soda-installer-import.service %{buildroot}%{_unitdir}/soda-installer-import.service
install -m 0644 %{_sourcedir}/soda-state-directories.service %{buildroot}%{_unitdir}/soda-state-directories.service
install -m 0644 %{_sourcedir}/var-srv-soda-projects.mount %{buildroot}%{_unitdir}/var-srv-soda-projects.mount
install -m 0644 %{_sourcedir}/opt-soda-toolchains.mount %{buildroot}%{_unitdir}/opt-soda-toolchains.mount
install -m 0644 %{_sourcedir}/90-soda.preset %{buildroot}%{_presetdir}/90-soda.preset
install -m 0644 %{_sourcedir}/10-soda-state.conf %{buildroot}%{_unitdir}/tailscaled.service.d/10-soda-state.conf
install -m 0644 %{_sourcedir}/10-soda-console.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
install -m 0644 %{_sourcedir}/soda.conf %{buildroot}%{_tmpfilesdir}/soda.conf
install -m 0644 %{_sourcedir}/60-soda-console.conf %{buildroot}%{_sysctldir}/60-soda-console.conf
install -m 0644 %{_sourcedir}/soda.sysusers %{buildroot}%{_sysusersdir}/soda.conf
install -m 0644 %{_sourcedir}/41-soda-project-accounts.conf %{buildroot}%{_sysconfdir}/ssh/sshd_config.d/41-soda-project-accounts.conf
install -m 0644 %{_sourcedir}/soda-console-welcome.sh %{buildroot}%{_sysconfdir}/profile.d/soda-console-welcome.sh

%files
%{_libexecdir}/soda/sodad
%{_libexecdir}/soda/soda-ssh
%{_bindir}/sodactl
%{_bindir}/soda-tailnet
%{_libexecdir}/soda/soda-console-welcome
%{_unitdir}/sodad.service
%{_unitdir}/soda-installer-import.service
%{_unitdir}/soda-state-directories.service
%{_unitdir}/var-srv-soda-projects.mount
%{_unitdir}/opt-soda-toolchains.mount
%{_presetdir}/90-soda.preset
%{_unitdir}/tailscaled.service.d/10-soda-state.conf
%{_unitdir}/getty@tty1.service.d/10-soda-console.conf
%{_tmpfilesdir}/soda.conf
%{_sysctldir}/60-soda-console.conf
%{_sysusersdir}/soda.conf
%config(noreplace) %{_sysconfdir}/ssh/sshd_config.d/41-soda-project-accounts.conf
%{_sysconfdir}/profile.d/soda-console-welcome.sh
%dir %attr(0755,root,root) %{_sysconfdir}/soda
%dir %attr(0755,root,root) %{_sysconfdir}/soda/authorized_keys
