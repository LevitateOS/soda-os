Name:           soda-tea
Version:        0.15.1
Release:        2%{?dist}
Summary:        Pinned Tea CLI for Soda OS development
License:        MIT
ExclusiveArch:  x86_64 aarch64

%description
The architecture-native Tea CLI for user-owned Forgejo operations, shipped as
immutable Soda OS image content. This package contains no downloader, updater,
configuration, service, or persistent runtime state.

%install
mkdir -p %{buildroot}%{_bindir} %{buildroot}%{_licensedir}/%{name}
install -m 0755 %{_sourcedir}/tea %{buildroot}%{_bindir}/tea
install -m 0644 %{_sourcedir}/tea-LICENSE %{buildroot}%{_licensedir}/%{name}/LICENSE

%check
test "$(uname -m)" = "%{_arch}"
%{buildroot}%{_bindir}/tea --version | grep -F "%{version}"
%{buildroot}%{_bindir}/tea --help >/dev/null

%files
%{_bindir}/tea
%license %{_licensedir}/%{name}/LICENSE
