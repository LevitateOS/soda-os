import type { ReactNode } from "react";
import { Content, Stack, StackItem, Title } from "@patternfly/react-core";
import { SodaEyebrow } from "../atoms/SodaEyebrow";
export function PageHeading({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <Stack hasGutter>
      <StackItem>
        <SodaEyebrow />
        <Title headingLevel="h1" id="page-title">
          {title}
        </Title>
      </StackItem>
      {description && (
        <StackItem>
          <Content>
            <p>{description}</p>
          </Content>
        </StackItem>
      )}
      {actions && <StackItem>{actions}</StackItem>}
    </Stack>
  );
}
