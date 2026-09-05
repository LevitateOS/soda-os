Name:           soda-cosign
Version:        3.1.3
Release:        1%{?dist}
Summary:        Pinned Cosign for Soda OS image verification
License:        Apache-2.0
ExclusiveArch:  x86_64 aarch64

%description
The upstream Cosign CLI, built natively from pinned source for verification of
Soda OS release signatures and provenance. No signing keys or credentials are
included.

%install
mkdir -p %{buildroot}%{_bindir} %{buildroot}%{_licensedir}/%{name}
install -m 0755 %{_sourcedir}/cosign %{buildroot}%{_bindir}/cosign
install -m 0644 %{_sourcedir}/cosign-LICENSE %{buildroot}%{_licensedir}/%{name}/LICENSE

%check
test "$(uname -m)" = "%{_arch}"
%{buildroot}%{_bindir}/cosign version --json | grep -F '"gitVersion": "v%{version}"'
%{buildroot}%{_bindir}/cosign verify --help >/dev/null
%{buildroot}%{_bindir}/cosign verify-attestation --help >/dev/null

%files
%{_bindir}/cosign
%license %{_licensedir}/%{name}/LICENSE
