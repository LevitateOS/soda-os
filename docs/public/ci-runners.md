A CI runner executes workflow jobs. Soda can create runners that execute
locally on the Soda machine, while Forgejo or GitHub remains responsible for
the workflow, job queue, repository permissions, secrets, and runner protocol.

Only a Soda administrator manages local runners. The **Runners** page in
Cockpit shows their exact count, status, and available local capacity.

## Use Forgejo with a local runner

1. Open **Runners** in Cockpit and create a local runner.
2. Register that runner with the bundled Forgejo service through Forgejo's
   native runner-registration flow.
3. Note the labels advertised by the registered runner.
4. Select those labels in the repository's Forgejo Actions workflow.
5. Run the workflow in Forgejo and confirm that the local runner accepts it.

Forgejo owns the workflow and reports the job result. Soda operates only the
runner executing on the Soda machine.

## Use GitHub with GitHub runners

Configure the repository's workflow and runner choice in GitHub. GitHub owns
and operates its runners, so no Soda runner setup is required.

## Use GitHub with a local runner

1. Open **Runners** in Cockpit and create a local runner.
2. Register it as a GitHub self-hosted runner through GitHub's native
   registration flow.
3. Note the labels GitHub assigns or the administrator selects.
4. Use those labels in the repository's GitHub Actions workflow.
5. Run the workflow in GitHub and confirm that the local runner accepts it.

GitHub continues to own the workflow, permissions, secrets, scheduling, and
job result. Soda shows and operates the runner because it executes locally.

## Manage local capacity

Use **Runners** to inspect local runner status and capacity and to perform the
available local runner lifecycle actions. Removing a local runner removes that
execution capacity; it does not delete the repository, workflow, or provider's
job history.
