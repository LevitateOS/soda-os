# Soda OS interactive installer overlay for Rocky Linux 10.2.
graphical
cdrom
firstboot --disable
selinux --enforcing
firewall --enabled --service=ssh --port=9090:tcp
services --enabled="sshd,sodad,soda-authd,soda-cockpit,avahi-daemon"
repo --name=soda --baseurl=file:///run/install/repo/soda/

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
/usr/bin/hostnamectl --root=/ set-hostname soda
/usr/bin/systemctl enable sshd sodad soda-authd soda-cockpit avahi-daemon
%end
