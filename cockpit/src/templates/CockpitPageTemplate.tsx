import type { ReactNode } from "react";
import { Page, PageSection, Stack, StackItem } from "@patternfly/react-core";
import { PageHeading } from "../molecules/PageHeading";
export function CockpitPageTemplate({
  title,
  description,
  actions,
  feedback,
  children,
  dialogs,
  busy,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  feedback?: ReactNode;
  children: ReactNode;
  dialogs?: ReactNode;
  busy: boolean;
}) {
  return (
    <Page sidebar={null} className="soda-page" mainAriaLabel={title} aria-busy={busy}>
      <PageSection>
        <Stack hasGutter>
          <StackItem>
            <PageHeading title={title} description={description} actions={actions} />
          </StackItem>
          {feedback && <StackItem>{feedback}</StackItem>}
        </Stack>
      </PageSection>
      {children}
      {dialogs}
    </Page>
  );
}
