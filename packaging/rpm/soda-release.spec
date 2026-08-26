Name:           soda-release
Version:        0.1.0
Release:        1%{?dist}
Summary:        Soda OS release identity
License:        MIT OR Apache-2.0
BuildArch:      noarch

%description
Release identity and defaults for the Soda OS Rocky Linux derivative.

%install
mkdir -p %{buildroot}%{_sysconfdir} %{buildroot}%{_prefix}/lib/soda %{buildroot}%{_datadir}/doc/soda-release %{buildroot}%{_datadir}/pixmaps %{buildroot}%{_datadir}/icons/hicolor/256x256/apps
cat > %{buildroot}%{_sysconfdir}/soda-release <<'EOF'
Soda OS release 0.1.0
EOF
cat > %{buildroot}%{_prefix}/lib/soda/os-release <<'EOF'
NAME="Soda OS"
VERSION="0.1.0"
ID="sodaos"
ID_LIKE="rhel centos fedora"
VERSION_ID="0.1.0"
PLATFORM_ID="platform:el10"
PRETTY_NAME="Soda OS 0.1.0"
ANSI_COLOR="0;38;2;16;215;232"
HOME_URL="https://github.com/LevitateOS/soda-os"
VARIANT="Remote Development Appliance"
VARIANT_ID="appliance"
EOF
cat > %{buildroot}%{_prefix}/lib/soda/issue <<'EOF'
Soda OS 0.1.0 \n \l

EOF
cat > %{buildroot}%{_prefix}/lib/soda/system-release <<'EOF'
Soda OS release 0.1.0
EOF
install -m 0644 %{_sourcedir}/BASE_SYSTEM.md %{buildroot}%{_datadir}/doc/soda-release/BASE_SYSTEM.md
install -m 0644 %{_sourcedir}/soda-symbol.svg %{buildroot}%{_datadir}/pixmaps/soda-os.svg
install -m 0644 %{_sourcedir}/soda-symbol-256.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/soda-os.png

%post
cp -f %{_prefix}/lib/soda/os-release %{_sysconfdir}/os-release
cp -f %{_prefix}/lib/soda/os-release %{_prefix}/lib/os-release
cp -f %{_prefix}/lib/soda/issue %{_sysconfdir}/issue
cp -f %{_prefix}/lib/soda/issue %{_sysconfdir}/issue.net
cp -f %{_prefix}/lib/soda/system-release %{_sysconfdir}/system-release
cp -f %{_prefix}/lib/soda/system-release %{_sysconfdir}/redhat-release

%posttrans
cp -f %{_prefix}/lib/soda/os-release %{_sysconfdir}/os-release
cp -f %{_prefix}/lib/soda/os-release %{_prefix}/lib/os-release
cp -f %{_prefix}/lib/soda/issue %{_sysconfdir}/issue
cp -f %{_prefix}/lib/soda/issue %{_sysconfdir}/issue.net
cp -f %{_prefix}/lib/soda/system-release %{_sysconfdir}/system-release
cp -f %{_prefix}/lib/soda/system-release %{_sysconfdir}/redhat-release

%files
%{_sysconfdir}/soda-release
%{_prefix}/lib/soda
%{_datadir}/doc/soda-release/BASE_SYSTEM.md
%{_datadir}/pixmaps/soda-os.svg
%{_datadir}/icons/hicolor/256x256/apps/soda-os.png
