Name:           soda-projects
Version:        %{soda_version}
Release:        2%{?dist}
Summary:        Soda OS Projects package for stock Cockpit
License:        MIT OR Apache-2.0
Requires:       cockpit-system, cockpit-ws, coreutils, git-core, glibc-common, openssh-clients, pam, policycoreutils, polkit, procps-ng, shadow-utils, soda-tea, systemd, tailscale, util-linux

%description
Soda OS branding, the focused stock-Cockpit Projects page, and its bounded
synchronous catalog and workspace lifecycle executables.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda \
    %{buildroot}%{_datadir}/cockpit/soda-projects %{buildroot}%{_datadir}/cockpit/branding/sodaos \
    %{buildroot}%{_datadir}/polkit-1/actions %{buildroot}%{_tmpfilesdir} %{buildroot}%{_sysusersdir} %{buildroot}%{_prefix}/lib/soda/pam
install -m 0755 %{_sourcedir}/soda-projects %{buildroot}%{_libexecdir}/soda/soda-projects
install -m 0755 %{_sourcedir}/soda-workspace-helper %{buildroot}%{_libexecdir}/soda/soda-workspace-helper
install -m 0644 %{_sourcedir}/soda-projects-manifest.json %{buildroot}%{_datadir}/cockpit/soda-projects/manifest.json
install -m 0644 %{_sourcedir}/soda-projects-index.html %{buildroot}%{_datadir}/cockpit/soda-projects/index.html
install -m 0644 %{_sourcedir}/soda-projects-app.mjs %{buildroot}%{_datadir}/cockpit/soda-projects/app.mjs
install -m 0644 %{_sourcedir}/soda-projects-protocol.mjs %{buildroot}%{_datadir}/cockpit/soda-projects/protocol.mjs
install -m 0644 %{_sourcedir}/soda-projects-ui.mjs %{buildroot}%{_datadir}/cockpit/soda-projects/ui.mjs
install -m 0644 %{_sourcedir}/soda-projects-app.css %{buildroot}%{_datadir}/cockpit/soda-projects/app.css
install -m 0644 %{_sourcedir}/soda-projects-branding.css %{buildroot}%{_datadir}/cockpit/branding/sodaos/branding.css
install -m 0644 %{_sourcedir}/soda-projects-symbol.svg %{buildroot}%{_datadir}/cockpit/branding/sodaos/soda-symbol.svg
install -m 0644 %{_sourcedir}/org.sodaos.projects.policy %{buildroot}%{_datadir}/polkit-1/actions/org.sodaos.projects.policy
install -m 0644 %{_sourcedir}/soda-projects.tmpfiles %{buildroot}%{_tmpfilesdir}/soda-projects.conf
install -m 0644 %{_sourcedir}/soda-projects.sysusers %{buildroot}%{_sysusersdir}/soda-projects.conf
install -m 0644 %{_sourcedir}/cockpit-stock.pam %{buildroot}%{_prefix}/lib/soda/pam/cockpit

%files
%{_libexecdir}/soda/soda-projects
%{_libexecdir}/soda/soda-workspace-helper
%{_datadir}/cockpit/soda-projects/
%{_datadir}/cockpit/branding/sodaos/
%{_datadir}/polkit-1/actions/org.sodaos.projects.policy
%{_tmpfilesdir}/soda-projects.conf
%{_sysusersdir}/soda-projects.conf
%{_prefix}/lib/soda/pam/cockpit
