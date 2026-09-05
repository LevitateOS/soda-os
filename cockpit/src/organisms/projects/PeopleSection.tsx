import { Button, PageSection, Stack, StackItem, Title } from "@patternfly/react-core";
export function PeopleSection({ busy, onRemove }: { busy: boolean; onRemove: () => void }) {
  return (
    <PageSection aria-labelledby="people-title">
      <Stack hasGutter>
        <Title headingLevel="h2" id="people-title">
          People
        </Title>
        <p>
          Stock Cockpit Accounts creates and lists primary Linux users and owns administrator
          status. Administrators can use this Soda-aware action to remove one: it deletes local Soda
          workspaces, then the primary Linux account. Forgejo account deletion remains separate
          inside Forgejo.
        </p>
        <StackItem>
          <Button variant="danger" isDisabled={busy} onClick={onRemove}>
            Remove person…
          </Button>
        </StackItem>
      </Stack>
    </PageSection>
  );
}
