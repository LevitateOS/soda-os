import { HelperText, HelperTextItem } from "@patternfly/react-core";
import { FormGroup, TextArea, TextInput } from "@patternfly/react-core";
import type { Project } from "../../projects/types";
export function CatalogFields({
  action,
  project,
  busy,
}: {
  action: "add-existing" | "edit";
  project?: Project;
  busy: boolean;
}) {
  return (
    <>
      {action === "add-existing" && (
        <FormGroup label="Project ID" fieldId="project-id" isRequired>
          <TextInput
            id="project-id"
            name="id"
            isRequired
            isDisabled={busy}
            pattern="[a-z][a-z0-9-]{0,23}"
            maxLength={24}
            autoComplete="off"
            aria-describedby="project-id-help"
          />
          <HelperText>
            <HelperTextItem id="project-id-help">
              A stable lowercase name used for workspace paths.
            </HelperTextItem>
          </HelperText>
        </FormGroup>
      )}
      {action === "edit" && (
        <p>
          Project ID: <code>{project?.id}</code>
        </p>
      )}
      <FormGroup label="Display name" fieldId="display-name" isRequired>
        <TextInput
          id="display-name"
          name="display_name"
          defaultValue={project?.display_name ?? ""}
          isRequired
          isDisabled={busy}
          autoComplete="off"
        />
      </FormGroup>
      <FormGroup
        label="Canonical Git URL"
        fieldId="canonical-url"
        isRequired={action === "add-existing"}
      >
        <TextInput
          id="canonical-url"
          name="canonical_url"
          defaultValue={project?.canonical_url ?? ""}
          readOnlyVariant={action === "edit" ? "default" : undefined}
          isRequired={action === "add-existing"}
          isDisabled={busy}
          inputMode="url"
          autoComplete="off"
          placeholder="git@example.test:team/project.git"
          aria-describedby={action === "edit" ? "canonical-url-help" : undefined}
        />
        {action === "edit" && (
          <HelperText>
            <HelperTextItem id="canonical-url-help">
              To replace this URL, an administrator must remove the project and its local
              workspaces, then add the project again with the replacement SSH URL. Removing the
              project does not delete the canonical repository.
            </HelperTextItem>
          </HelperText>
        )}
      </FormGroup>
      <FormGroup label="Additional metadata" fieldId="additional-metadata">
        <TextArea
          id="additional-metadata"
          name="additional_metadata"
          rows={4}
          defaultValue={project ? JSON.stringify(project.catalog_metadata, null, 2) : ""}
          isDisabled={busy}
          placeholder={'{"team":"web"}'}
        />
        <HelperText>
          <HelperTextItem>
            {action === "add-existing" ? "Optional JSON object" : "JSON object"}
          </HelperTextItem>
        </HelperText>
      </FormGroup>
    </>
  );
}
