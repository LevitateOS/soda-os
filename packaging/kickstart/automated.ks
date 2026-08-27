# Test-only unattended installer. Never ship this artifact as a release image.
text
url --mirrorlist="https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=BaseOS-10"
lang en_US.UTF-8
keyboard us
timezone UTC --utc
network --bootproto=dhcp --device=link --activate --hostname=soda
rootpw --lock
user --name=soda-test --groups=wheel --password=soda-test --plaintext
zerombr
clearpart --all --initlabel
autopart --type=lvm
shutdown
firstboot --disable
selinux --enforcing
firewall --enabled --service=ssh --port=9090:tcp
services --enabled="sshd,sodad,soda-authd,soda-cockpit,avahi-daemon"
repo --name=AppStream --mirrorlist="https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=AppStream-10"
repo --name=soda --baseurl=file:///run/install/repo/soda/

%packages --exclude-weakdeps
@^minimal-environment
avahi
curl
git
openssh-server
soda-release
soda-runtime
soda-cockpit
sudo
%end

%post --erroronfail
/usr/bin/systemctl enable sshd sodad soda-authd soda-cockpit avahi-daemon srv-soda-projects.mount opt-soda-toolchains.mount
%end
