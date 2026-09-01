Name:           soda-cockpit
Version:        0.4.0
Release:        1%{?dist}
Summary:        Soda OS Cockpit composition and Projects package
License:        MIT OR Apache-2.0
Requires:       avahi, cockpit-system, cockpit-ws, coreutils, git-core, glibc-common, pam, policycoreutils, polkit, procps-ng, shadow-utils, soda-runtime = %{version}, systemd, util-linux

%description
Stock Cockpit composition, Soda OS branding, and the focused Projects package.
The pre-reset standalone dashboard remains installed during the verified route
transition and is removed only after the stock Cockpit workflow is proven.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_unitdir} %{buildroot}%{_sysconfdir}/avahi/services %{buildroot}%{_sysconfdir}/pam.d \
    %{buildroot}%{_datadir}/cockpit/soda-projects %{buildroot}%{_datadir}/cockpit/branding/sodaos \
    %{buildroot}%{_datadir}/polkit-1/actions %{buildroot}%{_tmpfilesdir} %{buildroot}%{_prefix}/lib/soda/pam
install -m 0755 %{_sourcedir}/soda-cockpit %{buildroot}%{_libexecdir}/soda/soda-cockpit
install -m 0755 %{_sourcedir}/soda-authd %{buildroot}%{_libexecdir}/soda/soda-authd
install -m 0755 %{_sourcedir}/soda-projects %{buildroot}%{_libexecdir}/soda/soda-projects
install -m 0755 %{_sourcedir}/soda-workspace-helper %{buildroot}%{_libexecdir}/soda/soda-workspace-helper
install -m 0644 %{_sourcedir}/soda-cockpit.service %{buildroot}%{_unitdir}/soda-cockpit.service
install -m 0644 %{_sourcedir}/soda-authd.service %{buildroot}%{_unitdir}/soda-authd.service
install -m 0644 %{_sourcedir}/soda-cockpit.avahi.service %{buildroot}%{_sysconfdir}/avahi/services/soda-cockpit.service
install -m 0644 %{_sourcedir}/soda-cockpit.pam %{buildroot}%{_sysconfdir}/pam.d/soda-cockpit
install -m 0644 %{_sourcedir}/soda-projects-manifest.json %{buildroot}%{_datadir}/cockpit/soda-projects/manifest.json
install -m 0644 %{_sourcedir}/soda-projects-index.html %{buildroot}%{_datadir}/cockpit/soda-projects/index.html
install -m 0644 %{_sourcedir}/soda-projects-app.mjs %{buildroot}%{_datadir}/cockpit/soda-projects/app.mjs
install -m 0644 %{_sourcedir}/soda-projects-protocol.mjs %{buildroot}%{_datadir}/cockpit/soda-projects/protocol.mjs
install -m 0644 %{_sourcedir}/soda-projects-ui.mjs %{buildroot}%{_datadir}/cockpit/soda-projects/ui.mjs
install -m 0644 %{_sourcedir}/soda-projects-app.css %{buildroot}%{_datadir}/cockpit/soda-projects/app.css
install -m 0644 %{_sourcedir}/soda-cockpit-branding.css %{buildroot}%{_datadir}/cockpit/branding/sodaos/branding.css
install -m 0644 %{_sourcedir}/soda-cockpit-symbol.svg %{buildroot}%{_datadir}/cockpit/branding/sodaos/soda-symbol.svg
install -m 0644 %{_sourcedir}/org.sodaos.projects.policy %{buildroot}%{_datadir}/polkit-1/actions/org.sodaos.projects.policy
install -m 0644 %{_sourcedir}/soda-projects.tmpfiles %{buildroot}%{_tmpfilesdir}/soda-projects.conf
install -m 0644 %{_sourcedir}/cockpit-stock.pam %{buildroot}%{_prefix}/lib/soda/pam/cockpit

%files
%{_libexecdir}/soda/soda-cockpit
%{_libexecdir}/soda/soda-authd
%{_libexecdir}/soda/soda-projects
%{_libexecdir}/soda/soda-workspace-helper
%{_unitdir}/soda-cockpit.service
%{_unitdir}/soda-authd.service
%{_sysconfdir}/avahi/services/soda-cockpit.service
%config(noreplace) %{_sysconfdir}/pam.d/soda-cockpit
%{_datadir}/cockpit/soda-projects/
%{_datadir}/cockpit/branding/sodaos/
%{_datadir}/polkit-1/actions/org.sodaos.projects.policy
%{_tmpfilesdir}/soda-projects.conf
%{_prefix}/lib/soda/pam/cockpit
