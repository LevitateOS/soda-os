Name:           soda-runtime
Version:        0.1.0
Release:        1%{?dist}
Summary:        Soda OS project runtime
License:        MIT OR Apache-2.0
Requires:       git-core, openssh-server, shadow-utils, sqlite-libs

%description
Privileged project daemon, administrator CLI, and SSH session gateway for Soda OS.

%pre
getent group soda-api >/dev/null || groupadd --system soda-api

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_bindir} %{buildroot}%{_unitdir}
install -m 0755 %{_sourcedir}/sodad %{buildroot}%{_libexecdir}/soda/sodad
install -m 0755 %{_sourcedir}/soda-ssh %{buildroot}%{_libexecdir}/soda/soda-ssh
install -m 0755 %{_sourcedir}/sodactl %{buildroot}%{_bindir}/sodactl
install -m 0644 %{_sourcedir}/sodad.service %{buildroot}%{_unitdir}/sodad.service

%post
%systemd_post sodad.service

%preun
%systemd_preun sodad.service

%postun
%systemd_postun_with_restart sodad.service

%files
%{_libexecdir}/soda/sodad
%{_libexecdir}/soda/soda-ssh
%{_bindir}/sodactl
%{_unitdir}/sodad.service
