import { useState } from "react";
import {
  Button,
  Dropdown,
  DropdownItem,
  DropdownList,
  Flex,
  MenuToggle,
} from "@patternfly/react-core";
import type {
  CurrentUser,
  ProjectAction,
  Project,
  WorkspaceInspection,
} from "../../projects/types";
import { projectRemovalHidden, workspaceReady } from "../../projects/ui";

export function ProjectActions({
  project,
  inspection,
  currentUser,
  busy,
  onAction,
}: {
  project: Project;
  inspection?: WorkspaceInspection;
  currentUser: CurrentUser;
  busy: boolean;
  onAction: (action: ProjectAction, project: Project) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const exists = inspection?.exists ?? project.workspace_exists;
  const label = workspaceReady(inspection)
    ? "Connection details"
    : exists
      ? "Review setup"
      : "Set up for me";
  function manage(action: ProjectAction) {
    setMenuOpen(false);
    onAction(action, project);
  }
  return (
    <Flex gap={{ default: "gapSm" }}>
      <Button
        variant="secondary"
        isDisabled={busy}
        onClick={() => onAction(exists ? "inspect" : "setup", project)}
        aria-label={`${label} — ${project.display_name}`}
      >
        {label}
      </Button>
      <Dropdown
        isOpen={menuOpen}
        onOpenChange={setMenuOpen}
        toggle={(ref) => (
          <MenuToggle
            ref={ref}
            variant="plainText"
            isExpanded={menuOpen}
            isDisabled={busy}
            aria-label={`Actions — ${project.display_name}`}
            onClick={() => setMenuOpen(!menuOpen)}
          >
            Actions
          </MenuToggle>
        )}
      >
        <DropdownList>
          <DropdownItem onClick={() => manage("edit")}>Edit project</DropdownItem>
          {exists && (
            <DropdownItem onClick={() => manage("remove-workspace")}>
              Remove my workspace
            </DropdownItem>
          )}
          {!projectRemovalHidden(currentUser) && (
            <DropdownItem onClick={() => manage("remove")}>Remove project</DropdownItem>
          )}
        </DropdownList>
      </Dropdown>
    </Flex>
  );
}
