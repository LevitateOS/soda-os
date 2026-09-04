%global __brp_mangle_shebangs %{nil}

Name:           soda-runners
Version:        %{soda_version}
Release:        1%{?dist}
Summary:        Soda OS local CI runner composition
License:        MIT AND (MIT OR Apache-2.0)
AutoReqProv:     no
Requires:       cockpit-system, coreutils, forgejo-runner, libicu, policycoreutils, polkit, shadow-utils, soda-projects, systemd

%description
The administrator-only stock-Cockpit Runners page and its bounded local Linux
account, service, status, capacity, and lifecycle composition for provider-owned
Forgejo and GitHub CI runners.

%install
mkdir -p %{buildroot}%{_libexecdir}/soda %{buildroot}%{_datadir}/cockpit/soda-runners \
    %{buildroot}%{_datadir}/polkit-1/actions %{buildroot}%{_tmpfilesdir} %{buildroot}%{_sysusersdir} \
    %{buildroot}%{_unitdir} %{buildroot}%{_prefix}/lib/soda/github-actions-runner %{buildroot}%{_datadir}/soda-runners
install -m 0755 %{_sourcedir}/soda-runners %{buildroot}%{_libexecdir}/soda/soda-runners
install -m 0755 %{_sourcedir}/soda-runner-helper %{buildroot}%{_libexecdir}/soda/soda-runner-helper
install -m 0755 %{_sourcedir}/soda-runner-launch %{buildroot}%{_libexecdir}/soda/soda-runner-launch
install -m 0644 %{_sourcedir}/soda-runners-manifest.json %{buildroot}%{_datadir}/cockpit/soda-runners/manifest.json
install -m 0644 %{_sourcedir}/soda-runners-index.html %{buildroot}%{_datadir}/cockpit/soda-runners/index.html
install -m 0644 %{_sourcedir}/soda-runners-app.mjs %{buildroot}%{_datadir}/cockpit/soda-runners/app.mjs
install -m 0644 %{_sourcedir}/soda-runners-protocol.mjs %{buildroot}%{_datadir}/cockpit/soda-runners/protocol.mjs
install -m 0644 %{_sourcedir}/soda-runners-ui.mjs %{buildroot}%{_datadir}/cockpit/soda-runners/ui.mjs
install -m 0644 %{_sourcedir}/soda-runners-app.css %{buildroot}%{_datadir}/cockpit/soda-runners/app.css
install -m 0644 %{_sourcedir}/org.sodaos.runners.policy %{buildroot}%{_datadir}/polkit-1/actions/org.sodaos.runners.policy
install -m 0644 %{_sourcedir}/soda-runners.tmpfiles %{buildroot}%{_tmpfilesdir}/soda-runners.conf
install -m 0644 %{_sourcedir}/soda-runners.sysusers %{buildroot}%{_sysusersdir}/soda-runners.conf
install -m 0644 %{_sourcedir}/soda-runner@.service %{buildroot}%{_unitdir}/soda-runner@.service
tar -xzf %{_sourcedir}/github-actions-runner.tar.gz -C %{buildroot}%{_prefix}/lib/soda/github-actions-runner
printf '%s\n' '2.337.0' > %{buildroot}%{_datadir}/soda-runners/github-version

%files
%{_libexecdir}/soda/soda-runners
%{_libexecdir}/soda/soda-runner-helper
%{_libexecdir}/soda/soda-runner-launch
%{_datadir}/cockpit/soda-runners/
%{_datadir}/polkit-1/actions/org.sodaos.runners.policy
%{_tmpfilesdir}/soda-runners.conf
%{_sysusersdir}/soda-runners.conf
%{_unitdir}/soda-runner@.service
%{_prefix}/lib/soda/github-actions-runner/
%{_datadir}/soda-runners/github-version
