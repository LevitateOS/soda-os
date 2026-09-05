import { Button, Form, Modal, ModalHeader, ModalBody, ModalFooter } from "@patternfly/react-core";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
import { CatalogFields } from "../../molecules/projects/CatalogFields";
import type { Project } from "../../projects/types";
import type { ProjectDialogProps } from "./dialogTypes";
export function CatalogProjectDialog({
  action,
  project,
  busy,
  error,
  metadataError,
  onClose,
  onSubmit,
}: ProjectDialogProps & {
  action: "add-existing" | "edit";
  project?: Project;
  metadataError: { message: string } | null;
}) {
  const title = action === "add-existing" ? "Add repository" : "Edit project";
  const button = action === "add-existing" ? "Add repository" : "Save changes";
  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="project-dialog-title"
      onClose={busy ? undefined : onClose}
      onEscapePress={onClose}
    >
      <ModalHeader title={title} labelId="project-dialog-title" />
      <ModalBody>
        <Form id="project-action" onSubmit={onSubmit}>
          {action === "edit" && <input type="hidden" name="id" value={project?.id ?? ""} />}
          <CatalogFields
            action={action}
            project={project}
            busy={busy}
            metadataError={metadataError}
          />
          {error && <DiagnosticAlert message={error} role="alert" />}
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" isDisabled={busy} onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant="primary"
          type="submit"
          form="project-action"
          isDisabled={busy}
          isLoading={busy}
        >
          {button}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
