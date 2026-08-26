Name:           soda-cockpit
Version:        0.1.0
Release:        1%{?dist}
Summary:        Soda OS development cockpit
License:        MIT OR Apache-2.0
Requires:       avahi, pam

%description
Standalone Go and HTMX management website for Soda OS.

%pre
getent group soda-api >/dev/null || groupadd --system soda-api
getent passwd soda-cockpit >/dev/null || useradd --system --gid soda-api --home-dir /var/lib/soda --shell /sbin/nologin soda-cockpit

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_unitdir} %{buildroot}%{_sysconfdir}/avahi/services
install -m 0755 %{_sourcedir}/soda-cockpit %{buildroot}%{_libexecdir}/soda/soda-cockpit
install -m 0644 %{_sourcedir}/soda-cockpit.service %{buildroot}%{_unitdir}/soda-cockpit.service
install -m 0644 %{_sourcedir}/soda-cockpit.avahi.service %{buildroot}%{_sysconfdir}/avahi/services/soda-cockpit.service

%post
%systemd_post soda-cockpit.service

%preun
%systemd_preun soda-cockpit.service

%postun
%systemd_postun_with_restart soda-cockpit.service

%files
%{_libexecdir}/soda/soda-cockpit
%{_unitdir}/soda-cockpit.service
%{_sysconfdir}/avahi/services/soda-cockpit.service
