Name:           soda-installer-branding
Version:        0.1.0
Release:        1%{?dist}
Summary:        Soda OS Anaconda product overlay
License:        MIT OR Apache-2.0
BuildArch:      noarch

%description
Build-only assets and configuration for the Soda OS Anaconda product image.

%install
mkdir -p %{buildroot}%{_datadir}/soda-installer/product %{buildroot}%{_datadir}/soda-installer/manifests
cp -a %{_sourcedir}/soda-installer-product/. %{buildroot}%{_datadir}/soda-installer/product/
install -m 0644 %{_sourcedir}/branding.toml %{buildroot}%{_datadir}/soda-installer/manifests/branding.toml
install -m 0644 %{_sourcedir}/upstream.toml %{buildroot}%{_datadir}/soda-installer/manifests/upstream.toml

%files
%{_datadir}/soda-installer
