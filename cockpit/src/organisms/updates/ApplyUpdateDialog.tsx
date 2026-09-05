import {
  Button,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  ModalVariant,
} from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import type { Selection } from "../../updates/types";

export function ApplyUpdateDialog({
  selection,
  busy,
  onClose,
  onApply,
}: {
  selection: Selection | null;
  busy: boolean;
  onClose: () => void;
  onApply: () => void;
}) {
  return (
    <Modal
      variant={ModalVariant.small}
      isOpen={Boolean(selection)}
      onClose={onClose}
      aria-labelledby="updates-restart-title"
    >
      <ModalHeader title="Apply update and restart?" labelId="updates-restart-title" />
      <ModalBody>
        <p>
          Soda OS {selection?.version} will be enabled for the next boot and the server will
          restart. SSH sessions and running development workloads will be interrupted.
        </p>
        <p>
          Do not run other bootc or deployment commands while applying. The target is rechecked, but
          bootc does not provide an atomic expected-digest activation guard.
        </p>
        <p>
          <CodeValue>{selection?.reference}</CodeValue>
        </p>
      </ModalBody>
      <ModalFooter>
        <Button variant="danger" isDisabled={busy} onClick={onApply}>
          Apply and restart
        </Button>
        <Button variant="link" onClick={onClose}>
          Keep working
        </Button>
      </ModalFooter>
    </Modal>
  );
}
