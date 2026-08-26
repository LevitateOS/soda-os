# Test-only unattended installer. Never ship this artifact as a release image.
text
cdrom
lang en_US.UTF-8
keyboard us
timezone UTC --utc
network --bootproto=dhcp --device=link --activate --hostname=soda-test
rootpw --lock
user --name=soda-test --groups=wheel --password=soda-test --plaintext
zerombr
clearpart --all --initlabel
autopart --type=lvm
reboot
firstboot --disable
selinux --enforcing
firewall --enabled --service=ssh --port=9090:tcp
services --enabled="sshd,sodad,soda-cockpit,avahi-daemon"

%packages
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
/usr/bin/systemctl enable sshd sodad soda-cockpit avahi-daemon
%end
