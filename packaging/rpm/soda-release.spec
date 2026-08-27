Name:           soda-release
Version:        0.2.0
Release:        1%{?dist}
Summary:        Soda OS release identity
License:        MIT OR Apache-2.0
BuildArch:      noarch

%description
Release identity and defaults for the Soda OS Fedora bootc derivative.

%install
mkdir -p %{buildroot}%{_sysconfdir} %{buildroot}%{_prefix}/lib/soda %{buildroot}%{_datadir}/doc/soda-release %{buildroot}%{_datadir}/pixmaps
for size in 16 24 32 48 64 128 256 512; do
  mkdir -p %{buildroot}%{_datadir}/icons/hicolor/${size}x${size}/apps
done
cat > %{buildroot}%{_sysconfdir}/soda-release <<'EOF'
Soda OS release 0.2.0
EOF
cat > %{buildroot}%{_prefix}/lib/soda/os-release <<'EOF'
NAME="Soda OS"
VERSION="0.2.0"
ID="sodaos"
ID_LIKE="fedora"
VERSION_ID="0.2.0"
PLATFORM_ID="platform:f44"
PRETTY_NAME="Soda OS 0.2.0"
ANSI_COLOR="0;38;2;16;215;232"
HOME_URL="https://github.com/LevitateOS/soda-os"
VARIANT="Remote Development Appliance"
VARIANT_ID="appliance"
EOF
cat > %{buildroot}%{_prefix}/lib/soda/issue <<'EOF'
Soda OS 0.2.0 \n \l

EOF
cat > %{buildroot}%{_prefix}/lib/soda/system-release <<'EOF'
Soda OS release 0.2.0
EOF
install -m 0644 %{_sourcedir}/BASE_SYSTEM.md %{buildroot}%{_datadir}/doc/soda-release/BASE_SYSTEM.md
install -m 0644 %{_sourcedir}/soda-symbol.svg %{buildroot}%{_datadir}/pixmaps/soda-os.svg
for size in 16 24 32 48 64 128 256 512; do
  install -m 0644 %{_sourcedir}/soda-os-${size}.png %{buildroot}%{_datadir}/icons/hicolor/${size}x${size}/apps/soda-os.png
done

%files
%{_sysconfdir}/soda-release
%{_prefix}/lib/soda
%{_datadir}/doc/soda-release/BASE_SYSTEM.md
%{_datadir}/pixmaps/soda-os.svg
%{_datadir}/icons/hicolor/*/apps/soda-os.png
