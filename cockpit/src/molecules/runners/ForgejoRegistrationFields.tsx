import { HelperText, HelperTextItem } from "@patternfly/react-core";
import { FormGroup, Stack, TextInput } from "@patternfly/react-core";
import { ExternalLink } from "../../atoms/ExternalLink";
import { forgejoBrowserURL } from "../../runners/ui";
export function ForgejoRegistrationFields({
  active,
  busy,
  hostname,
}: {
  active: boolean;
  busy: boolean;
  hostname: string;
}) {
  return (
    <div hidden={!active}>
      <Stack hasGutter>
        <HelperText>
          <HelperTextItem>
            Create a system runner in{" "}
            <ExternalLink href={`${forgejoBrowserURL(hostname)}/admin/actions/runners`}>
              Forgejo runner administration
            </ExternalLink>
            , then copy its UUID and confidential token here.
          </HelperTextItem>
        </HelperText>
        <FormGroup label="Forgejo runner UUID" fieldId="registration-id" isRequired={active}>
          <TextInput
            id="registration-id"
            name="registration_id"
            isRequired={active}
            isDisabled={busy || !active}
            pattern="[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
            autoComplete="off"
          />
        </FormGroup>
        <FormGroup label="Native Forgejo host labels" fieldId="forgejo-labels" isRequired={active}>
          <TextInput
            id="forgejo-labels"
            name="forgejo_labels"
            defaultValue="soda-linux:host"
            isRequired={active}
            isDisabled={busy || !active}
            pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,63}:host(,[A-Za-z0-9][A-Za-z0-9._-]{0,63}:host)*"
            autoComplete="off"
          />
          <HelperText>
            <HelperTextItem>
              Use comma-separated <code>name:host</code> labels. Host jobs run directly as this
              runner's isolated Linux account.
            </HelperTextItem>
          </HelperText>
        </FormGroup>
      </Stack>
    </div>
  );
}
