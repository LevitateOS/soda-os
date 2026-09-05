import { Button, Form, Modal, ModalHeader, ModalBody, ModalFooter } from "@patternfly/react-core";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
import { ConfirmationField } from "../../molecules/ConfirmationField";
import type { Project } from "../../projects/types";
import { dialogCopy } from "../../projects/ui";
import type { ProjectDialogProps } from "./dialogTypes";
export function RemoveProjectDialog({
  action,
  project,
  busy,
  error,
  onClose,
  onSubmit,
}: ProjectDialogProps & { action: "remove" | "remove-workspace"; project?: Project }) {
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
          <input type="hidden" name="id" value={project?.id ?? ""} />
          <ConfirmationField
            id="project-confirmation"
            label={
              <>
                Type <code>{project?.id}</code> to confirm
              </>
            }
            busy={busy}
          />
          {error && <DiagnosticAlert message={error} role="alert" />}
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" isDisabled={busy} onClick={onClose}>
          {error ? "Close" : "Cancel"}
        </Button>
        <Button
          variant="danger"
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
