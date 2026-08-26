Name:           soda-release
Version:        0.1.0
Release:        1%{?dist}
Summary:        Soda OS release identity
License:        MIT OR Apache-2.0
BuildArch:      noarch

%description
Release identity and defaults for the Soda OS Rocky Linux derivative.

%install
mkdir -p %{buildroot}%{_sysconfdir} %{buildroot}%{_prefix}/lib/soda
cat > %{buildroot}%{_sysconfdir}/soda-release <<'EOF'
Soda OS release 0.1.0 (Rocky Linux 10.2)
EOF
cat > %{buildroot}%{_prefix}/lib/soda/os-release <<'EOF'
NAME="Soda OS"
VERSION="0.1.0 (Rocky Linux 10.2)"
ID="sodaos"
ID_LIKE="rhel centos fedora"
VERSION_ID="0.1.0"
PLATFORM_ID="platform:el10"
PRETTY_NAME="Soda OS 0.1.0"
ANSI_COLOR="0;38;2;53;132;228"
HOME_URL="https://github.com/LevitateOS/soda-os"
EOF

%post
cp -f %{_prefix}/lib/soda/os-release %{_sysconfdir}/os-release

%files
%{_sysconfdir}/soda-release
%{_prefix}/lib/soda/os-release
