import type { ReactNode } from "react";
import { FormGroup, TextInput } from "@patternfly/react-core";
export function ConfirmationField({
  id,
  label,
  busy,
}: {
  id: string;
  label: ReactNode;
  busy: boolean;
}) {
  return (
    <FormGroup label={label} fieldId={id} isRequired>
      <TextInput id={id} name="confirmation" isRequired isDisabled={busy} autoComplete="off" />
    </FormGroup>
  );
}
