# CI runners

Use provider workflows with provider-hosted capacity or a runner executing locally on Soda.

Local runner jobs execute repository code. Each has one job slot, an
unprivileged Linux account, no `sudo` or human-home access, persistent files,
and network access. Use one only for repositories and contributors you trust.

## Forgejo with a local runner

1. In Forgejo runner administration, create a system runner and copy its UUID
   and confidential token.
2. As a Soda administrator, open Cockpit **Runners**, select **Create local
   runner**, and choose **Bundled Forgejo**.
3. Enter a lowercase **Runner ID**, the Forgejo UUID and token, and one or more
   `name:host` labels. The default is `soda-linux:host`.
4. Select **Register and start**. Confirm that **Local capacity** shows the
   runner listening with one configured slot.
5. Select it from a Forgejo Actions workflow with the ordinary label, such as
   `runs-on: soda-linux`.

## GitHub with GitHub-hosted runners

Configure the workflow and GitHub-hosted runner entirely in GitHub. Soda needs
no setup, connection, or state for this path.

## GitHub with a local runner

1. In the repository, organization, or enterprise Actions settings, choose
   **New self-hosted runner** and copy the registration URL and short-lived
   token.
2. In Cockpit **Runners**, select **Create local runner**, choose **GitHub**,
   and enter those values with a Runner ID and custom labels.
3. Select **Register and start**, then confirm the listener and one-slot
   capacity. GitHub also adds `self-hosted`, `linux`, and the architecture
   label; workflows select the runner with ordinary `runs-on` labels.

## Operate or remove a local runner

**Runners** shows the provider, client version, listener status, architecture,
and capacity. Use **Start**, **Stop**, or **Restart** for its local service.

**Remove** permanently deletes the local Linux account, provider client state,
working files, dependencies, and uncommitted job changes. The provider record
and CI history remain in Forgejo or GitHub; remove the offline record there.

The GitHub client version comes from that runner's installed copy. Existing
GitHub runners keep their client across OS updates; a new runner uses the
client bundled with the updated OS. To replace an older client:

1. Let its current job finish and select **Stop**.
2. Preserve any local working files or dependencies you need, then select
   **Remove**. This permanently deletes the local runner's files.
3. Remove the old runner record in GitHub, obtain a fresh registration token,
   and create the replacement in **Runners**.
4. Check the reported client version and run a provider-native test job.

If GitHub registration succeeds but saving local runner details fails, the
error reports the local cleanup outcome. Inspect/remove any remaining GitHub
runner record before retrying. If local account removal fails, its state is
retained and the error names its path; resolve that native Linux failure
before retrying. Soda does not automatically remove the GitHub record.

Forgejo or GitHub owns registration, tokens, labels, workflows, scheduling,
results, and history. Soda owns only the runner executing on this machine.

## Next step

Continue with [Administration](../40-Operate-Soda-OS/10-administration.md).
