import { Button, Flex } from "@patternfly/react-core";
import type { CurrentUser, FormAction, Project } from "../../projects/types";
import { projectRemovalHidden } from "../../projects/ui";
export function ProjectActions({
  project,
  currentUser,
  busy,
  onAction,
}: {
  project: Project;
  currentUser: CurrentUser;
  busy: boolean;
  onAction: (action: FormAction, project: Project) => void;
}) {
  return (
    <Flex gap={{ default: "gapSm" }}>
      <Button size="sm" isDisabled={busy} onClick={() => onAction("setup", project)}>
        Set up for me
      </Button>
      {project.workspace_exists && (
        <Button
          size="sm"
          variant="link"
          isDanger
          isDisabled={busy}
          onClick={() => onAction("remove-workspace", project)}
        >
          Remove my workspace
        </Button>
      )}
      <Button
        size="sm"
        variant="secondary"
        isDisabled={busy}
        onClick={() => onAction("edit", project)}
      >
        Edit
      </Button>
      {!projectRemovalHidden(currentUser) && (
        <Button
          size="sm"
          variant="link"
          isDanger
          isDisabled={busy}
          onClick={() => onAction("remove", project)}
        >
          Remove project
        </Button>
      )}
    </Flex>
  );
}
