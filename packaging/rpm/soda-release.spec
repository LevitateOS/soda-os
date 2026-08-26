Name:           soda-release
Version:        0.1.0
Release:        1%{?dist}
Summary:        Soda OS release identity
License:        MIT OR Apache-2.0
BuildArch:      noarch

%description
Release identity and defaults for the Soda OS Rocky Linux derivative.

%install
mkdir -p %{buildroot}%{_sysconfdir}
cat > %{buildroot}%{_sysconfdir}/soda-release <<'EOF'
Soda OS release 0.1.0 (Rocky Linux 10.2)
EOF

%files
%{_sysconfdir}/soda-release
