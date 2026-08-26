# Soda OS interactive installer overlay for Rocky Linux 10.2.
graphical
url --mirrorlist="https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=BaseOS-10"
rootpw --lock
network --bootproto=dhcp --device=link --activate --hostname=soda
firstboot --disable
selinux --enforcing
firewall --enabled --service=ssh --port=9090:tcp
services --enabled="sshd,sodad,soda-authd,soda-cockpit,avahi-daemon"
repo --name=AppStream --mirrorlist="https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=AppStream-10"
repo --name=soda --baseurl=file:///run/install/repo/soda/

%packages --exclude-weakdeps
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
/usr/bin/systemctl enable sshd sodad soda-authd soda-cockpit avahi-daemon
%end
