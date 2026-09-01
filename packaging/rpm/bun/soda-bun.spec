Name:           soda-bun
Version:        1.4.0
Release:        1%{?dist}
Summary:        Pinned Bun runtime for Soda OS development
License:        LicenseRef-Bun-Upstream-Combined
ExclusiveArch:  x86_64 aarch64

%description
The architecture-native Bun runtime shipped as immutable Soda OS image content.
This package contains no downloader, updater, or persistent runtime state.

%install
mkdir -p %{buildroot}%{_bindir} %{buildroot}%{_licensedir}/%{name}
install -m 0755 %{_sourcedir}/bun %{buildroot}%{_bindir}/bun
install -m 0644 %{_sourcedir}/LICENSE.md %{buildroot}%{_licensedir}/%{name}/LICENSE.md

%check
test "$(%{_sourcedir}/bun --version)" = "%{version}"
test "$(%{_sourcedir}/bun -e 'process.stdout.write("soda-bun-native")')" = "soda-bun-native"

%files
%{_bindir}/bun
%license %{_licensedir}/%{name}/LICENSE.md
