import { useEffect, useRef, useState } from "react";
import {
  ExpandableSection,
  FormGroup,
  HelperText,
  HelperTextItem,
  TextArea,
  TextInput,
} from "@patternfly/react-core";
import type { Project } from "../../projects/types";

export function CatalogFields({
  action,
  project,
  busy,
  metadataError,
}: {
  action: "add-existing" | "edit";
  project?: Project;
  busy: boolean;
  metadataError: { message: string } | null;
}) {
  const [expanded, setExpanded] = useState(false);
  const metadata = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    if (!metadataError) return;
    setExpanded(true);
    const frame = requestAnimationFrame(() => metadata.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [metadataError]);
  return (
    <>
      <FormGroup label="Project name" fieldId="display-name" isRequired>
        <TextInput
          id="display-name"
          name="display_name"
          defaultValue={project?.display_name ?? ""}
          isRequired
          isDisabled={busy}
          autoComplete="off"
        />
      </FormGroup>
      <FormGroup label="Project ID" fieldId="project-id" isRequired={action === "add-existing"}>
        <TextInput
          id="project-id"
          name={action === "add-existing" ? "id" : undefined}
          defaultValue={project?.id ?? ""}
          readOnlyVariant={action === "edit" ? "default" : undefined}
          isRequired={action === "add-existing"}
          isDisabled={busy}
          pattern="[a-z][a-z0-9-]{0,23}"
          maxLength={24}
          autoComplete="off"
          aria-describedby={action === "add-existing" ? "project-id-help" : undefined}
        />
        {action === "add-existing" && (
          <HelperText>
            <HelperTextItem id="project-id-help">
              Up to 24 lowercase letters, digits, or hyphens; start with a letter.
            </HelperTextItem>
          </HelperText>
        )}
      </FormGroup>
      <FormGroup
        label="Repository SSH address"
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
          aria-describedby={action === "add-existing" ? "canonical-url-help" : undefined}
        />
        {action === "add-existing" && (
          <HelperText>
            <HelperTextItem id="canonical-url-help">
              Copy the SSH address from the repository’s clone menu.
            </HelperTextItem>
          </HelperText>
        )}
      </FormGroup>
      <p>
        The ID and address cannot be edited later. Replacing the address requires administrator
        removal of this project and its local workspaces. The Git-host repository stays.
      </p>
      <ExpandableSection
        toggleText="Additional metadata (optional)"
        isExpanded={expanded}
        onToggle={(_, value) => setExpanded(value)}
      >
        <FormGroup label="Metadata JSON" fieldId="additional-metadata">
          <TextArea
            ref={metadata}
            id="additional-metadata"
            name="additional_metadata"
            rows={3}
            defaultValue={project ? JSON.stringify(project.catalog_metadata, null, 2) : ""}
            isDisabled={busy}
            validated={metadataError ? "error" : "default"}
            aria-invalid={Boolean(metadataError)}
            aria-describedby="metadata-help"
          />
          <HelperText>
            <HelperTextItem
              id="metadata-help"
              variant={metadataError ? "error" : "default"}
              role={metadataError ? "alert" : undefined}
            >
              {metadataError?.message || 'Optional JSON object, for example {"team":"web"}.'}
            </HelperTextItem>
          </HelperText>
        </FormGroup>
      </ExpandableSection>
    </>
  );
}
