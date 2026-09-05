import { useState } from "react";
import {
  Alert,
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
  Stack,
  TextInput,
} from "@patternfly/react-core";
import type { PreviewProject } from "./RepositoryForm";

export function RemovalDialog({
  project,
  scope,
  partial,
  hasWorkspace,
  destination,
  onRemove,
  onClose,
  onDestination,
}: {
  project: PreviewProject;
  scope: "workspace" | "project";
  partial: boolean;
  hasWorkspace: boolean;
  destination: string;
  onRemove: () => void;
  onClose: () => void;
  onDestination: (destination: string) => void;
}) {
  const [confirmation, setConfirmation] = useState("");
  const [touched, setTouched] = useState(false);
  const matches = confirmation === project.id;
  const action = scope === "workspace" ? "Remove my workspace" : "Remove project";
  return (
    <Modal isOpen variant="small" aria-labelledby="removal-title" onClose={onClose}>
      <ModalHeader
        title={partial ? "Project removal is incomplete" : `${action}?`}
        labelId="removal-title"
      />
      <ModalBody>
        {partial ? (
          <Stack hasGutter>
            <Alert
              isInline
              variant="warning"
              title="Some local data was permanently deleted"
              role="alert"
            >
              Your workspace was deleted. Bob’s workspace and the shared project entry remain. The
              repository on the Git host was not deleted.
            </Alert>
            <p>
              An administrator needs to inspect Bob’s workspace account before continuing removal.
            </p>
            <ExpandableSection toggleText="Technical details">
              <p>Example: userdel could not remove soda-w-bob. No further accounts were deleted.</p>
            </ExpandableSection>
          </Stack>
        ) : (
          <Form
            id="preview-removal"
            onSubmit={(event) => {
              event.preventDefault();
              if (matches) onRemove();
            }}
          >
            <p>
              <strong>{project.name}</strong>
            </p>
            <p>
              {scope === "workspace"
                ? "Permanently delete your workspace and all its local files, including unpushed work. Your running tasks will stop."
                : `Permanently delete ${hasWorkspace ? "Alice’s and Bob’s local workspaces" : "Bob’s local workspace"}, including unpushed work, and remove this project from the list. Running tasks in those workspaces will stop.`}
            </p>
            <p>
              {scope === "workspace"
                ? "Other people’s workspaces and the shared project stay. "
                : ""}
              The repository on the Git host stays. Save, push, or copy needed work first. There is
              no undo.
            </p>
            <FormGroup
              label={`Type ${project.id} to confirm`}
              fieldId="preview-confirmation"
              isRequired
            >
              <TextInput
                id="preview-confirmation"
                value={confirmation}
                onChange={(_, value) => setConfirmation(value)}
                onBlur={() => setTouched(true)}
                aria-describedby="confirmation-help"
                aria-invalid={touched && !matches}
                validated={touched && !matches ? "error" : "default"}
                autoComplete="off"
              />
              <HelperText>
                <HelperTextItem
                  id="confirmation-help"
                  variant={touched && !matches ? "error" : "default"}
                  role={touched && !matches ? "alert" : undefined}
                >
                  {touched && !matches
                    ? `Enter ${project.id} exactly to enable removal.`
                    : "Removal is enabled only when the project ID matches."}
                </HelperTextItem>
              </HelperText>
            </FormGroup>
          </Form>
        )}
        {destination && (
          <p role="status">Preview destination: {destination}. No external action was performed.</p>
        )}
      </ModalBody>
      <ModalFooter>
        {partial ? (
          <>
            <Button
              onClick={() =>
                onDestination(
                  "Cockpit Accounts → soda-w-bob (Bob’s workspace) → inspect before retrying project removal",
                )
              }
            >
              Inspect remaining workspace
            </Button>
            <Button variant="link" onClick={onClose}>
              Close
            </Button>
          </>
        ) : (
          <>
            <Button variant="danger" type="submit" form="preview-removal" isDisabled={!matches}>
              {action}
            </Button>
            <Button variant="link" onClick={onClose}>
              Cancel
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
