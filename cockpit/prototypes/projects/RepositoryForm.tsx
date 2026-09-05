import { useState, type FormEvent } from "react";
import {
  Button,
  ExpandableSection,
  Form,
  FormGroup,
  HelperText,
  HelperTextItem,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  TextArea,
  TextInput,
} from "@patternfly/react-core";

export interface PreviewProject {
  name: string;
  id: string;
  address: string;
  metadata: string;
}

export const exampleProject: PreviewProject = {
  name: "Team website",
  id: "website",
  address: "git@github.com:example/website.git",
  metadata: "",
};

export function RepositoryForm({
  project,
  onSave,
  onClose,
}: {
  project?: PreviewProject;
  onSave: (project: PreviewProject) => void;
  onClose: () => void;
}) {
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [advanced, setAdvanced] = useState(false);
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const next: PreviewProject = {
      name: ((data.get("name") as string | null) ?? "").trim(),
      id: ((data.get("id") as string | null) ?? "").trim(),
      address: ((data.get("address") as string | null) ?? "").trim(),
      metadata: (data.get("metadata") as string | null) ?? "",
    };
    const invalid: Record<string, string> = {};
    if (!next.name) invalid.name = "Enter a project name.";
    if (!/^[a-z][a-z0-9-]{0,23}$/.test(next.id)) {
      invalid.id =
        "Start with a lowercase letter. Use up to 24 lowercase letters, digits, or hyphens.";
    }
    // Illustrative preview validation, not authoritative Git URL validation.
    if (!next.address.startsWith("ssh://") && !/^[^\s@]+@[^\s:]+:.+$/.test(next.address)) {
      invalid.address = "Use the SSH clone address, for example git@github.com:team/website.git.";
    }
    try {
      const metadata: unknown = next.metadata.trim() ? JSON.parse(next.metadata) : {};
      if (!metadata || typeof metadata !== "object" || Array.isArray(metadata)) {
        invalid.metadata = "Enter a JSON object, or leave this blank.";
      } else if (
        ["id", "display_name", "canonical_url"].some((key) => Object.hasOwn(metadata, key))
      ) {
        invalid.metadata = "Keep the project ID, name, and repository address in their own fields.";
      }
    } catch {
      invalid.metadata = 'Use valid JSON, for example {"team":"web"}.';
    }
    setErrors(invalid);
    const first = Object.keys(invalid)[0];
    if (first) {
      if (first === "metadata") setAdvanced(true);
      requestAnimationFrame(() => (form.elements.namedItem(first) as HTMLElement)?.focus());
      return;
    }
    onSave(next);
  }
  return (
    <Modal isOpen variant="medium" aria-labelledby="repository-title" onClose={onClose}>
      <ModalHeader title={project ? "Edit project" : "Add repository"} labelId="repository-title" />
      <ModalBody>
        <Form id="preview-repository" onSubmit={submit} noValidate>
          <FormGroup label="Project name" fieldId="preview-name" isRequired>
            <TextInput
              id="preview-name"
              name="name"
              defaultValue={project?.name}
              isRequired
              validated={errors.name ? "error" : "default"}
              aria-invalid={Boolean(errors.name)}
              aria-describedby="name-help"
            />
            <HelperText>
              <HelperTextItem
                id="name-help"
                variant={errors.name ? "error" : "default"}
                role={errors.name ? "alert" : undefined}
              >
                {errors.name || "The name your team will see."}
              </HelperTextItem>
            </HelperText>
          </FormGroup>
          <FormGroup label="Project ID" fieldId="preview-id" isRequired>
            <TextInput
              id="preview-id"
              name="id"
              defaultValue={project?.id}
              isRequired
              maxLength={24}
              readOnlyVariant={project ? "default" : undefined}
              validated={errors.id ? "error" : "default"}
              aria-invalid={Boolean(errors.id)}
              aria-describedby="id-help"
            />
            <HelperText>
              <HelperTextItem
                id="id-help"
                variant={errors.id ? "error" : "default"}
                role={errors.id ? "alert" : undefined}
              >
                {errors.id ||
                  "Up to 24 lowercase letters, digits, or hyphens; start with a letter."}
              </HelperTextItem>
            </HelperText>
          </FormGroup>
          <FormGroup label="Repository SSH address" fieldId="preview-address" isRequired>
            <TextInput
              id="preview-address"
              name="address"
              defaultValue={project?.address}
              isRequired
              readOnlyVariant={project ? "default" : undefined}
              validated={errors.address ? "error" : "default"}
              aria-invalid={Boolean(errors.address)}
              aria-describedby="address-help"
            />
            <HelperText>
              <HelperTextItem
                id="address-help"
                variant={errors.address ? "error" : "default"}
                role={errors.address ? "alert" : undefined}
              >
                {errors.address || "Copy the SSH address from the repository’s clone menu."}
              </HelperTextItem>
            </HelperText>
          </FormGroup>
          <p>
            The ID and address cannot be edited later. Replacing the address requires deleting this
            project’s local workspaces. The repository on the Git host stays.
          </p>
          <ExpandableSection
            toggleText="Additional metadata (optional)"
            isExpanded={advanced}
            onToggle={(_, expanded) => setAdvanced(expanded)}
          >
            <FormGroup label="Metadata JSON" fieldId="preview-metadata">
              <TextArea
                id="preview-metadata"
                name="metadata"
                rows={3}
                defaultValue={project?.metadata}
                validated={errors.metadata ? "error" : "default"}
                aria-invalid={Boolean(errors.metadata)}
                aria-describedby="metadata-help"
              />
              <HelperText>
                <HelperTextItem
                  id="metadata-help"
                  variant={errors.metadata ? "error" : "default"}
                  role={errors.metadata ? "alert" : undefined}
                >
                  {errors.metadata || 'Optional JSON object, for example {"team":"web"}.'}
                </HelperTextItem>
              </HelperText>
            </FormGroup>
          </ExpandableSection>
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button type="submit" form="preview-repository">
          {project ? "Save changes" : "Add repository"}
        </Button>
        <Button variant="link" onClick={onClose}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}
