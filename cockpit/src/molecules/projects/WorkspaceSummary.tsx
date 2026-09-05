import { CodeValue } from "../../atoms/CodeValue";
import type { Project } from "../../projects/types";
import { sshCommand } from "../../projects/ui";
export function WorkspaceSummary({ project, hostname }: { project: Project; hostname: string }) {
  return (
    <>
      {" "}
      <span>{project.workspace_exists ? "Workspace account exists" : "No workspace account"}</span>
      <CodeValue>{sshCommand(project.workspace_username, hostname)}</CodeValue>
    </>
  );
}
