import type { FormEvent } from "react";
import { Button, Form, Modal, ModalBody, ModalFooter, ModalHeader } from "@patternfly/react-core";
import { ConfirmationField } from "../../molecules/ConfirmationField";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
export function RemoveRunnerDialog({
  id,
  busy,
  error,
  onClose,
  onSubmit,
}: {
  id: string;
  busy: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="remove-title"
      aria-describedby="remove-description"
      onClose={busy ? undefined : onClose}
      onEscapePress={onClose}
    >
      <ModalHeader
        title="Remove local runner"
        labelId="remove-title"
        descriptorId="remove-description"
        description="This permanently stops the listener and deletes its Linux account, provider client state, working files, dependencies, and uncommitted job changes. Its provider record and CI history remain with Forgejo or GitHub; remove the offline record there."
      />
      <ModalBody>
        <Form id="remove-runner" onSubmit={onSubmit}>
          <ConfirmationField
            id="runner-confirmation"
            label={
              <>
                Type <code>{id}</code> to confirm
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
        <Button variant="danger" type="submit" form="remove-runner" isDisabled={busy}>
          Remove local runner
        </Button>
      </ModalFooter>
    </Modal>
  );
}
