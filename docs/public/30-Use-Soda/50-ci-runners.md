# CI runners

Use provider-hosted CI capacity or register local Forgejo and GitHub runners through Cockpit's Runners page.

Local jobs execute repository code with persistent files and network access.
Each runner has one configured job slot and its own unprivileged Linux account,
without sudo or access to human homes. Use local capacity only for repositories
and contributors you trust. Providers own workflows, scheduling, logs, and history.

## Register a local Forgejo runner

You need Linux administrative access in Cockpit and permission to register
runners in the bundled Forgejo instance. These privileges are independent;
see [Forgejo administration](30-forgejo.md#create-a-forgejo-administrator).

1. In Forgejo runner administration, create a system runner and copy its UUID
   and confidential token. Follow [Forgejo runner guidance](https://forgejo.org/docs/latest/admin/actions/).
2. Open **Cockpit → Runners → Create local runner** and choose **Bundled Forgejo**.
3. Enter a lowercase **Runner ID**, the Forgejo UUID/token, and comma-separated
   `name:host` labels. The default is `soda-linux:host`.
4. Select **Register and start**; confirm **Local capacity** shows a listening
   runner with one configured slot.
5. Select its ordinary label in a Forgejo Actions workflow, for example
   `runs-on: soda-linux`, and inspect the job result in Forgejo.

A local listener starting is not evidence that the provider scheduled or passed
a job. Check both sides after registration.

## GitHub-hosted runners

Configure [GitHub-hosted runners](https://docs.github.com/en/actions/using-github-hosted-runners)
within GitHub. No Soda runner setup or retained Soda state is needed for that path.

## Register a local GitHub runner

1. In the repository, organization, or enterprise Actions settings, choose
   **New self-hosted runner** and obtain the registration URL and short-lived
   token. Follow [GitHub's registration procedure](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners/adding-self-hosted-runners).
2. In Cockpit **Runners**, select **Create local runner**, choose **GitHub**, and
   enter those values, a Runner ID, and custom labels.
3. Select **Register and start**, then confirm listening state and one slot.
4. Use provider-native `runs-on` labels in your workflow. GitHub also adds
   `self-hosted`, `linux`, and the architecture label. Confirm a test job in GitHub.

Never put registration tokens in screenshots, project metadata, workflow files,
or shared logs. The browser clears submitted secrets; native clients retain the
registration material they need in their dedicated local state.

## Start, stop, and inspect

The page shows provider, client version, listener state, architecture, and capacity.
Use **Start**, **Stop**, or **Restart** for its local service. Let active jobs
finish before maintenance; stopping a listener can interrupt work. Job scheduling
and history remain in Forgejo/GitHub, not the local capacity table.

For service failures, use Cockpit Services/Logs and the corresponding
`soda-runner@RUNNER_ID.service` journal. For rejected registration, expired tokens,
labels, or unscheduled jobs, inspect the provider's registration and workflow.

## Remove or replace a runner

**Remove permanently deletes** the local account, provider client state,
working files, dependencies, and uncommitted job changes. Preserve needed data
first. The provider registration and history remain; remove the offline record
at the provider separately.

Existing GitHub runners keep their installed client across OS updates. New
runners use the client bundled with the updated OS. To replace one:

1. Let its job finish, stop it, and preserve needed local files.
2. Remove it locally, then remove the old GitHub registration.
3. Obtain fresh registration input, create the replacement, and check its client
   version and a real provider job.

If registration succeeds but local publication fails, read the reported cleanup
result and inspect any remaining provider record before retrying. A failed local
account removal can retain state at the reported path; resolve that native Linux
problem first. Soda does not silently delete remote records or retry registration.

Include local runner data in [backup planning](../50-Operate/30-backups-and-restoration.md)
when it cannot be reconstructed from the provider and repository.
