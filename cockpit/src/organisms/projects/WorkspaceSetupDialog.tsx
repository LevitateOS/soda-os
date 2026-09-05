import { Button, Form, Modal, ModalHeader, ModalBody, ModalFooter } from "@patternfly/react-core";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
import type { Project } from "../../projects/types";
import { dialogCopy } from "../../projects/ui";
import type { ProjectDialogProps } from "./dialogTypes";
export function WorkspaceSetupDialog({
  project,
  busy,
  error,
  onClose,
  onSubmit,
}: ProjectDialogProps & { project?: Project }) {
  const action = "setup";
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
          <>
            <p>
              Project: <strong>{project?.display_name}</strong>
            </p>
            <p>
              Soda creates an SSH keypair only in this workspace and uses it for this clone. The
              authoritative Git host owns access. If clone authentication fails, Soda reports the
              public key for you to register with that host before retrying. Configure tools with{" "}
              <code>mise</code> and authenticate Tea and GitHub CLI directly inside the workspace.
            </p>
          </>
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
