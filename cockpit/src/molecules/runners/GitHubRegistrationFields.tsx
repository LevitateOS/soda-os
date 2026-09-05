import { HelperText, HelperTextItem } from "@patternfly/react-core";
import { FormGroup, Stack, TextInput } from "@patternfly/react-core";
export function GitHubRegistrationFields({ active, busy }: { active: boolean; busy: boolean }) {
  return (
    <div hidden={!active}>
      <Stack hasGutter>
        <HelperText>
          <HelperTextItem>
            In the repository, organization, or enterprise Actions settings, choose{" "}
            <strong>New self-hosted runner</strong> and copy the short-lived registration token.
          </HelperTextItem>
        </HelperText>
        <FormGroup label="GitHub registration URL" fieldId="registration-url" isRequired={active}>
          <TextInput
            id="registration-url"
            name="registration_url"
            type="url"
            isRequired={active}
            isDisabled={busy || !active}
            placeholder="https://github.com/owner/repository"
            autoComplete="off"
          />
        </FormGroup>
        <FormGroup label="Custom GitHub labels" fieldId="github-labels" isRequired={active}>
          <TextInput
            id="github-labels"
            name="github_labels"
            defaultValue="soda-local"
            isRequired={active}
            isDisabled={busy || !active}
            pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,63}(,[A-Za-z0-9][A-Za-z0-9._-]{0,63})*"
            autoComplete="off"
          />
          <HelperText>
            <HelperTextItem>
              GitHub also adds its native <code>self-hosted</code>, <code>linux</code>, and
              architecture labels.
            </HelperTextItem>
          </HelperText>
        </FormGroup>
      </Stack>
    </div>
  );
}
