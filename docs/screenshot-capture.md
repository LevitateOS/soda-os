# Handbook screenshot capture brief

Unpublished release work. Real captures are supplied by the operator; do not
substitute generated UI, marketing illustrations, or empty published image links.
Remove completed rows as verified captures are integrated. [The handbook authoring
contract](public/README.md) owns asset syntax; the website owns rendering.

## Capture conditions

Use a release-candidate interface and disposable, representative account/project
names. Hide private keys, tokens, passwords, authentication URLs, personal email,
private repository names, and sensitive terminal/browser details **before capture**.
Do not obscure a required control or alter the UI to imply a capability. Capture
the relevant page with readable controls and enough navigation to identify it;
avoid huge full-desktop images. PNG is suitable for UI text; JPEG/WebP are accepted.

Record the source release and viewport in the review, not the published filename.
A screenshot demonstrates the visible interface only, not that its action passed.
Compare every caption/alt draft below with the delivered image before publishing.
Do not crop away a warning to make a dangerous action look simpler.

## Requested captures

Paths are relative to `docs/public/assets/`. Use numbered variants only when the
handbook actually compares states; keep one canonical image per instruction.

| File | Capture / handbook location | Draft alt text |
| --- | --- | --- |
| `cockpit/overview.png` | Overview with Soda navigation; Cockpit guide after page map | Cockpit Overview with Projects, Runners, Tailscale, and Soda Updates in the navigation |
| `cockpit/authorized-keys.png` | Disposable primary account's public-key control; First connection | Accounts view showing where to add an authorized public SSH key |
| `cockpit/projects.png` | Catalog with one ready workspace and one not set up; Projects introduction | Projects catalog with individual workspace setup and connection actions |
| `cockpit/workspace-key.png` | Setup waiting for outbound public-key registration; Projects setup | Workspace setup showing the public Git key to register and Retry setup action |
| `cockpit/workspace-connection.png` | Confirmed complete clone and connection details; Projects result | Verified workspace username, SSH command, and repository path |
| `cockpit/tailscale.png` | Connected disposable device; Tailscale sign-in result | Tailscale page showing a connected device and its private address |
| `cockpit/runners.png` | One listening disposable runner, no registration token; Runners | Local runner capacity showing provider, client version, and listener state |
| `cockpit/updates.png` | Verified downloaded release before apply; Updates | Soda Updates showing downloaded release details and Apply and restart action |
| `cockpit/removal-review.png` | Disposable project's destructive review, warnings visible; Removal | Project removal review listing affected local workspaces and irreversible deletion warning |
| `forgejo/repository.png` | Native repository view and SSH clone control; Forgejo create | Forgejo repository showing its SSH clone address |
| `forgejo/ssh-keys.png` | Native user public-key settings; Forgejo registration | Forgejo SSH key settings for registering a workspace public key |

## Acceptance

Confirm file content matches the approved release interface and contains no
secrets; check title, control names, scope, and draft alt/caption. Add the image
only where it materially clarifies the step. Commit source prose and assets
before website sync. Verify built asset URLs, narrow/wide layouts, and both themes.
Until supplied and reviewed, screenshots and their real-image visual acceptance
remain release work—not broken placeholders in the published handbook.
