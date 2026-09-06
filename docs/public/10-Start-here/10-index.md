# Soda OS documentation

Install Soda, connect from your preferred client, and develop in your own Linux project workspaces.

Use a powerful computer as a complete remote environment or as additional
capacity alongside your everyday computer. For teams, one shared server runs
builds, editors, agents, databases, and development processes while each
person-project workspace has its own account, home, and clone.

## Start with your task

| You want to… | Start here |
| --- | --- |
| Understand accounts and shared resources | [Product model](20-product-model.md) |
| Install on a computer or from an ISO in a VM | [Install on premises](../20-Deploy/20-install-on-premises.md) |
| Import a reusable VM image or deploy on Scaleway | [Deploy to a cloud or VM](../20-Deploy/10-deploy-to-cloud.md) |
| Connect to a newly installed server | [First connection](../20-Deploy/30-first-connection.md) |
| Find your project and create a workspace | [Projects and workspaces](../30-Use-Soda/20-projects-and-workspaces.md) |
| Open your editor, use Git, or install tools | [Connect and develop](../40-Develop/10-connect-and-develop.md) |
| Add a teammate | [People and access](../50-Operate/10-people-and-access.md) |
| Diagnose, protect, or maintain the server | [Administration](../50-Operate/20-administration.md) |

## The path to your first workspace

1. [Verify the download](../20-Deploy/05-verify-downloads.md) for your x86-64 or
   AArch64 machine, then install with Anaconda or provision the QCOW2 with cloud-init.
2. Log in at the console and [make the first connection](../20-Deploy/30-first-connection.md)
   through your trusted LAN or Tailscale.
3. Open [Cockpit](../30-Use-Soda/10-cockpit.md), Soda's dashboard for administration.
4. Create a repository in [Forgejo](../30-Use-Soda/30-forgejo.md), or use an
   existing repository at your Git host, and add its SSH URL to Projects.
5. Select **Set up for me**. Register the workspace public key with the Git host
   when requested, then retry to complete the clone.
6. Copy the workspace SSH command, connect your editor, and use mise directly
   for the repository's development tools.

You need console access for initial installation, a personal SSH key, and a
trusted LAN or Tailnet. Cloud services remain private; public SSH is not a
bootstrap step. You do not need a Soda source checkout or developer tools to
install or use Soda.

## Find a Soda page

The dashboard guides cover [Projects](../30-Use-Soda/20-projects-and-workspaces.md),
[Runners](../30-Use-Soda/50-ci-runners.md),
[Tailscale](../30-Use-Soda/40-tailscale.md), and
[Soda Updates](../30-Use-Soda/60-updates-and-fallback.md).
[Forgejo](../30-Use-Soda/30-forgejo.md) is the separate built-in Git website.

## Windows and WSL2

WSL2 support for x86-64 Windows gaming PCs is planned for a future release,
with no WSL2 download. Use the ISO or QCOW2 deployment guides for hardware and
virtual-machine installations.
