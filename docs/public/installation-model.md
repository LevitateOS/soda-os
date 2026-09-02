Installation is designed to leave an ordinary Linux administrator, a
same-named initial Forgejo administrator, SSH access, and private Tailnet
reachability without creating a permanent Soda bootstrap system.

## Product contract

Every installation uses an image built for the machine's architecture. AArch64
and x86-64 are equal product targets; neither is a fallback for the other.

Stock graphical Anaconda owns storage, networking, bootloader setup, bootc
deployment, and native Linux-account creation. A separate protected removable
input medium supplies exactly four values:

- administrator username;
- administrator password;
- administrator SSH public key; and
- one-use Tailscale authentication key.

Anaconda creates the primary Linux account, adds it to `wheel`, sets its Linux
password, and installs its SSH public key. A fixed installation-only handoff
creates the same-named initial Forgejo site administrator. On first boot, a
small one-attempt service gives the temporary key to Tailscale, deletes the key,
and disables itself.

The protected medium contains secrets and must be ejected and removed before
installation continues. Soda keeps no bootstrap database, credential service,
retry job, public onboarding endpoint, or alternate account system afterward.

Once installed, the owner reaches OpenSSH and Cockpit through the Tailnet. See
[connect and develop](connect-and-develop.md) for the private-access model.

## Current implementation

The repository builds architecture-specific local OCI archives and installer
ISOs. The current installer uses stock Fedora 44 Anaconda plus fixed
installation-only input and finalization hooks. Normal installations retain
Anaconda's interactive storage and networking workflow; only the disposable
acceptance harness supplies an unattended destructive storage recipe.

A fresh native x86-64 installation has exercised the protected medium,
administrator creation, initial Forgejo administrator, one-use Tailscale
enrollment, credential removal, stock Cockpit, Projects, direct workspace SSH,
and the installed toolset together. The same current path still requires
matching-native AArch64 repetition before release-level completion.

Soda OS currently publishes no tag, release, downloadable ISO, or supported
end-user installation command. Local candidate generation is a maintainer and
development workflow, not a public installation path.
