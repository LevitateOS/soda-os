# Soda OS interactive installer overlay for Rocky Linux 10.2.
graphical
cdrom
lang en_US.UTF-8
keyboard us
timezone Europe/Brussels --utc
network --bootproto=dhcp --device=link --activate --hostname=soda
firstboot --disable
selinux --enforcing
firewall --enabled --service=ssh --port=9090:tcp
services --enabled="sshd,sodad,soda-cockpit,avahi-daemon"

%packages
@^minimal-environment
avahi
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
