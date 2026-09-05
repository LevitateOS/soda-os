import { Button, Form, Modal, ModalHeader, ModalBody, ModalFooter } from "@patternfly/react-core";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
import { CatalogFields } from "../../molecules/projects/CatalogFields";
import type { Project } from "../../projects/types";
import { dialogCopy } from "../../projects/ui";
import type { ProjectDialogProps } from "./dialogTypes";
export function CatalogProjectDialog({
  action,
  project,
  busy,
  error,
  onClose,
  onSubmit,
}: ProjectDialogProps & { action: "add-existing" | "edit"; project?: Project }) {
  const [title, description, button] = dialogCopy[action];
  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="project-dialog-title"
      aria-describedby="project-dialog-description"
      onClose={busy ? undefined : onClose}
      onEscapePress={onClose}
    >
      <ModalHeader
        title={title}
        labelId="project-dialog-title"
        description={description}
        descriptorId="project-dialog-description"
      />
      <ModalBody>
        <Form id="project-action" onSubmit={onSubmit}>
          {action === "edit" && <input type="hidden" name="id" value={project?.id ?? ""} />}
          <CatalogFields action={action} project={project} busy={busy} />
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
