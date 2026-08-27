Name:           soda-cockpit
Version:        0.2.0
Release:        1%{?dist}
Summary:        Soda OS development cockpit
License:        MIT OR Apache-2.0
Requires:       avahi, pam, soda-runtime = %{version}

%description
Standalone Go and HTMX management website for Soda OS.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_unitdir} %{buildroot}%{_sysconfdir}/avahi/services %{buildroot}%{_sysconfdir}/pam.d
install -m 0755 %{_sourcedir}/soda-cockpit %{buildroot}%{_libexecdir}/soda/soda-cockpit
install -m 0755 %{_sourcedir}/soda-authd %{buildroot}%{_libexecdir}/soda/soda-authd
install -m 0644 %{_sourcedir}/soda-cockpit.service %{buildroot}%{_unitdir}/soda-cockpit.service
install -m 0644 %{_sourcedir}/soda-authd.service %{buildroot}%{_unitdir}/soda-authd.service
install -m 0644 %{_sourcedir}/soda-cockpit.avahi.service %{buildroot}%{_sysconfdir}/avahi/services/soda-cockpit.service
install -m 0644 %{_sourcedir}/soda-cockpit.pam %{buildroot}%{_sysconfdir}/pam.d/soda-cockpit

%post
%systemd_post soda-authd.service soda-cockpit.service

%preun
%systemd_preun soda-authd.service soda-cockpit.service

%postun
%systemd_postun_with_restart soda-authd.service soda-cockpit.service

%files
%{_libexecdir}/soda/soda-cockpit
%{_libexecdir}/soda/soda-authd
%{_unitdir}/soda-cockpit.service
%{_unitdir}/soda-authd.service
%{_sysconfdir}/avahi/services/soda-cockpit.service
%config(noreplace) %{_sysconfdir}/pam.d/soda-cockpit
