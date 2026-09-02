Name:           soda-forgejo
Version:        15.0.7
Release:        4%{?dist}
Summary:        Soda OS built-in Git service
License:        MIT AND GPL-3.0-or-later
Requires:       checkpolicy, git-core, git-lfs, pam, policycoreutils, shadow-utils, systemd, tailscale, util-linux-core

%description
Pinned PAM-enabled Forgejo runtime for the Soda OS built-in Git service.

%install
mkdir -p %{buildroot}%{_bindir} %{buildroot}%{_libexecdir}/soda %{buildroot}%{_unitdir} %{buildroot}%{_sysusersdir} %{buildroot}%{_tmpfilesdir} %{buildroot}%{_datadir}/soda/forgejo %{buildroot}%{_datadir}/soda/selinux %{buildroot}%{_sysconfdir}/pam.d
install -m 0755 %{_sourcedir}/forgejo %{buildroot}%{_bindir}/forgejo
install -m 0755 %{_sourcedir}/forgejo-init %{buildroot}%{_libexecdir}/soda/forgejo-init
install -m 0755 %{_sourcedir}/forgejo-tailnet %{buildroot}%{_libexecdir}/soda/forgejo-tailnet
install -m 0644 %{_sourcedir}/forgejo.service %{buildroot}%{_unitdir}/forgejo.service
install -m 0644 %{_sourcedir}/forgejo-init.service %{buildroot}%{_unitdir}/forgejo-init.service
install -m 0644 %{_sourcedir}/forgejo.sysusers %{buildroot}%{_sysusersdir}/forgejo.conf
install -m 0644 %{_sourcedir}/forgejo.tmpfiles %{buildroot}%{_tmpfilesdir}/forgejo.conf
install -m 0644 %{_sourcedir}/forgejo-app.ini.tmpl %{buildroot}%{_datadir}/soda/forgejo/app.ini.tmpl
install -m 0644 %{_sourcedir}/soda-forgejo.pam %{buildroot}%{_sysconfdir}/pam.d/soda-forgejo
install -m 0644 %{_sourcedir}/soda-forgejo-shadow.te %{buildroot}%{_datadir}/soda/selinux/soda-forgejo-shadow.te

%files
%{_bindir}/forgejo
%{_libexecdir}/soda/forgejo-init
%{_libexecdir}/soda/forgejo-tailnet
%{_unitdir}/forgejo.service
%{_unitdir}/forgejo-init.service
%{_sysusersdir}/forgejo.conf
%{_tmpfilesdir}/forgejo.conf
%{_datadir}/soda/forgejo/app.ini.tmpl
%{_datadir}/soda/selinux/soda-forgejo-shadow.te
%config(noreplace) %{_sysconfdir}/pam.d/soda-forgejo
