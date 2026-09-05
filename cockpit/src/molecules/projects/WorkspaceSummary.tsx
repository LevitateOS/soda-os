import { Label } from "@patternfly/react-core";
import type { Project, WorkspaceInspection } from "../../projects/types";
import { workspaceReady } from "../../projects/ui";

export function WorkspaceSummary({
  project,
  inspection,
}: {
  project: Project;
  inspection?: WorkspaceInspection;
}) {
  if (workspaceReady(inspection)) return <Label color="green">Ready</Label>;
  if (inspection?.checkout_problem || inspection?.workspace_key_problem)
    return <Label color="orange">Needs attention</Label>;
  if (inspection?.exists) return <Label>Setup incomplete</Label>;
  if (inspection?.primary_key_problem) return <Label>SSH key needed</Label>;
  return (
    <Label>
      {(inspection?.exists ?? project.workspace_exists) ? "Setup not confirmed" : "Not set up"}
    </Label>
  );
}
