# Soda OS documentation

Install a Soda machine, connect your team, create isolated workspaces, and operate the system safely.

Soda OS turns one Linux machine into a shared remote development system. Each
person has a normal Linux account for identity and administration, while daily
development happens in a separate workspace for each person and project.

## Choose where to start

- **Infrastructure owner:** start with [Deploy to a cloud](../20-Deploy/10-deploy-to-cloud.md)
  or [Install on premises](../20-Deploy/20-install-on-premises.md).
- **Administrator:** complete [Make the first connection](../20-Deploy/30-first-connection.md),
  then [add people and manage access](../30-Develop/10-people-and-access.md).
- **Developer:** start with [Projects and workspaces](../30-Develop/20-projects-and-workspaces.md),
  then [Connect and develop](../30-Develop/30-connect-and-develop.md).
- **System operator:** use [Administration](../40-Operate-Soda-OS/10-administration.md),
  [Updates and fallback](../40-Operate-Soda-OS/20-updates-and-fallback.md), and
  [Data safety and removal](../40-Operate-Soda-OS/30-data-safety-and-removal.md).

Read [Product model](20-product-model.md) first if you want to understand the
accounts, services, and ownership boundaries behind these tasks.

## From download to development

1. Open [Deploy to a cloud](../20-Deploy/10-deploy-to-cloud.md) or
   [Install on premises](../20-Deploy/20-install-on-premises.md), then choose
   the artifact for the machine's architecture.
2. Install with the network ISO or import the reusable QCOW2.
3. Complete **Soda Setup** from the machine console.
4. Connect over the LAN or Tailscale with SSH, Cockpit, and Forgejo.
5. Add each later primary account through stock Cockpit or native Linux, then
   let that person sign in to Forgejo normally and manage repository keys there.
6. Create the repository in its native Git host, then add it to Cockpit's
   **Projects** page.
7. Select **Set up for me**, register the reported workspace key with the Git
   host if asked, and retry to complete your isolated workspace.
8. Connect directly to that workspace with OpenSSH.
9. Use `mise` directly in the workspace to install the tools required by you or
   the project.

## Prerequisites

- An x86-64 or AArch64 machine or virtual machine.
- A disk whose contents may be replaced during installation.
- Network access while using the network installer.
- Console access for installation and Soda Setup.
- One SSH public key for the first administrator.
- Either a trusted LAN or a Tailscale network.

Cloud deployments require a usable VM console. Soda does not use public SSH as
an onboarding path.

## Expected result

You finish with a remotely accessible Soda machine, a primary administrator,
native Forgejo access, and a private development workspace reached through
ordinary SSH.

## If something fails

Keep the exact error shown by Anaconda, Soda Setup, Cockpit, Forgejo, or
the command you ran. Soda reports native failures instead of hiding them behind
a background workflow. Retry only the failed task after correcting its stated
cause.

## Next step

Read [Product model](20-product-model.md), or go directly to the deployment
guide for your environment.
