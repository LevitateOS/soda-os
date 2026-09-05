import { HelperText, HelperTextItem } from "@patternfly/react-core";
import { useCallback, useRef, useState, type FormEvent } from "react";
import {
  Button,
  Form,
  FormGroup,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Radio,
  TextInput,
} from "@patternfly/react-core";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
import { ForgejoRegistrationFields } from "../../molecules/runners/ForgejoRegistrationFields";
import { GitHubRegistrationFields } from "../../molecules/runners/GitHubRegistrationFields";
export function RegisterRunnerDialog({
  busy,
  onClose,
  onSubmit,
  hostname,
  error,
}: {
  busy: boolean;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  hostname: string;
  error: string;
}) {
  const [provider, setProvider] = useState("forgejo");
  const token = useRef<HTMLInputElement | null>(null);
  const tokenRef = useCallback((node: HTMLInputElement | null) => {
    if (!node && token.current) token.current.value = "";
    token.current = node;
  }, []);
  // Keep the token solely in the native input; clear even if Cockpit tears down the page.

  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="registration-title"
      aria-describedby="registration-description"
      onClose={busy ? undefined : onClose}
      onEscapePress={onClose}
    >
      <ModalHeader
        title="Create local runner"
        labelId="registration-title"
        descriptorId="registration-description"
        description="The provider creates the registration input. Soda passes it only to that provider's runner."
      />
      <ModalBody>
        <Form id="register-runner" onSubmit={onSubmit}>
          <FormGroup label="Runner ID" fieldId="runner-id" isRequired>
            <TextInput
              id="runner-id"
              name="id"
              isRequired
              isDisabled={busy}
              pattern="[a-z][a-z0-9-]{0,15}"
              maxLength={16}
              autoComplete="off"
              aria-describedby="runner-id-help"
            />
            <HelperText>
              <HelperTextItem id="runner-id-help">
                A stable lowercase local name. It is also the provider runner name for GitHub.
              </HelperTextItem>
            </HelperText>
          </FormGroup>
          <FormGroup label="Provider" role="group" fieldId="provider">
            <Radio
              id="forgejo"
              name="provider"
              value="forgejo"
              label="Bundled Forgejo"
              isChecked={provider === "forgejo"}
              isDisabled={busy}
              onChange={() => setProvider("forgejo")}
            />
            <Radio
              id="github"
              name="provider"
              value="github"
              label="GitHub"
              isChecked={provider === "github"}
              isDisabled={busy}
              onChange={() => setProvider("github")}
            />
          </FormGroup>
          <ForgejoRegistrationFields
            active={provider === "forgejo"}
            busy={busy}
            hostname={hostname}
          />
          <GitHubRegistrationFields active={provider === "github"} busy={busy} />
          <FormGroup label="Provider registration token" fieldId="registration-token" isRequired>
            <TextInput
              ref={tokenRef}
              id="registration-token"
              name="registration_token"
              type="password"
              isRequired
              isDisabled={busy}
              autoComplete="off"
            />
            <HelperText>
              <HelperTextItem>
                Used only through the provider's native registration input. Soda descriptors,
                command arguments, environment, and logs never contain it. The provider runner
                retains the credential it needs to reconnect.
              </HelperTextItem>
            </HelperText>
          </FormGroup>
          {error && <DiagnosticAlert message={error} role="alert" />}
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" isDisabled={busy} onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form="register-runner" isDisabled={busy} isLoading={busy}>
          Register and start
        </Button>
      </ModalFooter>
    </Modal>
  );
}
