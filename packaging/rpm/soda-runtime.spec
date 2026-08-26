Name:           soda-runtime
Version:        0.2.0
Release:        1%{?dist}
Summary:        Soda OS project runtime
License:        MIT OR Apache-2.0
Requires:       ca-certificates, gcc, gcc-c++, git-core, iproute, make, openssh-clients, openssh-server, policycoreutils, policycoreutils-python-utils, pkgconf-pkg-config, shadow-utils, sqlite-libs, systemd, tar, unzip, util-linux-core, xz

%description
Privileged project daemon, administrator CLI, and SSH session gateway for Soda OS.

%pre
getent group soda-api >/dev/null || groupadd --system soda-api
install -d -m 0755 /srv/soda /srv/soda/projects
install -d -m 0755 /opt/soda /opt/soda/toolchains
install -d -o root -g root -m 0755 /etc/soda /etc/soda/authorized_keys

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_sysconfdir}/ssh/sshd_config.d %{buildroot}%{_sysconfdir}/soda/authorized_keys
install -m 0755 %{_sourcedir}/sodad %{buildroot}%{_libexecdir}/soda/sodad
install -m 0755 %{_sourcedir}/soda-ssh %{buildroot}%{_libexecdir}/soda/soda-ssh
install -m 0755 %{_sourcedir}/sodactl %{buildroot}%{_bindir}/sodactl
install -m 0644 %{_sourcedir}/sodad.service %{buildroot}%{_unitdir}/sodad.service
install -m 0644 %{_sourcedir}/40-soda-observability.conf %{buildroot}%{_sysconfdir}/ssh/sshd_config.d/40-soda-observability.conf
install -m 0644 %{_sourcedir}/41-soda-project-accounts.conf %{buildroot}%{_sysconfdir}/ssh/sshd_config.d/41-soda-project-accounts.conf

%post
%systemd_post sodad.service
semanage fcontext -a -e /home /srv/soda/projects 2>/dev/null || :
semanage fcontext -a -t ssh_home_t '/etc/soda/authorized_keys(/.*)?' 2>/dev/null || semanage fcontext -m -t ssh_home_t '/etc/soda/authorized_keys(/.*)?' 2>/dev/null || :
restorecon -RF /srv/soda/projects 2>/dev/null || :
restorecon -RF /etc/soda/authorized_keys 2>/dev/null || :
/usr/sbin/sshd -t
systemctl try-reload-or-restart sshd.service >/dev/null 2>&1 || :

%preun
%systemd_preun sodad.service

%postun
%systemd_postun_with_restart sodad.service

%files
%{_libexecdir}/soda/sodad
%{_libexecdir}/soda/soda-ssh
%{_bindir}/sodactl
%{_unitdir}/sodad.service
%config(noreplace) %{_sysconfdir}/ssh/sshd_config.d/40-soda-observability.conf
%config(noreplace) %{_sysconfdir}/ssh/sshd_config.d/41-soda-project-accounts.conf
%dir %attr(0755,root,root) %{_sysconfdir}/soda
%dir %attr(0755,root,root) %{_sysconfdir}/soda/authorized_keys
