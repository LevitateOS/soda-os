import { FormGroup, Stack, TextInput } from "@patternfly/react-core";
export function GitHubRegistrationFields({ active, busy }: { active: boolean; busy: boolean }) {
  return (
    <div hidden={!active}>
      <Stack hasGutter>
        <p>
          In the repository, organization, or enterprise Actions settings, choose{" "}
          <strong>New self-hosted runner</strong> and copy the short-lived registration token.
        </p>
        <FormGroup label="GitHub registration URL" fieldId="registration-url" isRequired={active}>
          <TextInput
            id="registration-url"
            name="registration_url"
            type="url"
            isRequired={active}
            isDisabled={busy}
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
            isDisabled={busy}
            pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,63}(,[A-Za-z0-9][A-Za-z0-9._-]{0,63})*"
            autoComplete="off"
          />
          <p>
            GitHub also adds its native <code>self-hosted</code>, <code>linux</code>, and
            architecture labels.
          </p>
        </FormGroup>
      </Stack>
    </div>
  );
}
