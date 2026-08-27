Name:           soda-runtime
Version:        0.3.1
Release:        1%{?dist}
Summary:        Soda OS project runtime
License:        MIT OR Apache-2.0
Requires:       ca-certificates, gcc, gcc-c++, git-core, iproute, make, openssh-clients, openssh-server, policycoreutils, policycoreutils-python-utils, pkgconf-pkg-config, shadow-utils, sqlite-libs, systemd, tar, unzip, util-linux-core, xz

%description
Privileged project daemon, administrator CLI, and SSH session gateway for Soda OS.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_presetdir} %{buildroot}%{_tmpfilesdir} %{buildroot}%{_sysusersdir} %{buildroot}%{_sysconfdir}/ssh/sshd_config.d %{buildroot}%{_sysconfdir}/soda/authorized_keys
install -m 0755 %{_sourcedir}/sodad %{buildroot}%{_libexecdir}/soda/sodad
install -m 0755 %{_sourcedir}/soda-ssh %{buildroot}%{_libexecdir}/soda/soda-ssh
install -m 0755 %{_sourcedir}/sodactl %{buildroot}%{_bindir}/sodactl
install -m 0755 %{_sourcedir}/cosign %{buildroot}%{_libexecdir}/soda/cosign
install -m 0644 %{_sourcedir}/sodad.service %{buildroot}%{_unitdir}/sodad.service
install -m 0644 %{_sourcedir}/soda-state-directories.service %{buildroot}%{_unitdir}/soda-state-directories.service
install -m 0644 %{_sourcedir}/var-srv-soda-projects.mount %{buildroot}%{_unitdir}/var-srv-soda-projects.mount
install -m 0644 %{_sourcedir}/opt-soda-toolchains.mount %{buildroot}%{_unitdir}/opt-soda-toolchains.mount
install -m 0644 %{_sourcedir}/90-soda.preset %{buildroot}%{_presetdir}/90-soda.preset
install -m 0644 %{_sourcedir}/soda.conf %{buildroot}%{_tmpfilesdir}/soda.conf
install -m 0644 %{_sourcedir}/soda.sysusers %{buildroot}%{_sysusersdir}/soda.conf
install -m 0644 %{_sourcedir}/41-soda-project-accounts.conf %{buildroot}%{_sysconfdir}/ssh/sshd_config.d/41-soda-project-accounts.conf

%files
%{_libexecdir}/soda/sodad
%{_libexecdir}/soda/soda-ssh
%{_bindir}/sodactl
%{_libexecdir}/soda/cosign
%{_unitdir}/sodad.service
%{_unitdir}/soda-state-directories.service
%{_unitdir}/var-srv-soda-projects.mount
%{_unitdir}/opt-soda-toolchains.mount
%{_presetdir}/90-soda.preset
%{_tmpfilesdir}/soda.conf
%{_sysusersdir}/soda.conf
%config(noreplace) %{_sysconfdir}/ssh/sshd_config.d/41-soda-project-accounts.conf
%dir %attr(0755,root,root) %{_sysconfdir}/soda
%dir %attr(0755,root,root) %{_sysconfdir}/soda/authorized_keys
