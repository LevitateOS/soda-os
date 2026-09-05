import { useState } from "react";
import {
  Button,
  Dropdown,
  DropdownItem,
  DropdownList,
  Flex,
  MenuToggle,
  PageSection,
} from "@patternfly/react-core";

export function PeopleSection({
  busy,
  administrator,
  onRemove,
}: {
  busy: boolean;
  administrator: boolean;
  onRemove: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <PageSection aria-label="People management">
      <Flex alignItems={{ default: "alignItemsCenter" }}>
        <Button
          variant="link"
          isInline
          isDisabled={busy}
          onClick={() => window.cockpit.jump("/users")}
        >
          Manage people in Accounts
        </Button>
        {administrator && (
          <Dropdown
            isOpen={open}
            onOpenChange={setOpen}
            toggle={(ref) => (
              <MenuToggle
                ref={ref}
                variant="plainText"
                isExpanded={open}
                isDisabled={busy}
                onClick={() => setOpen(!open)}
              >
                People actions
              </MenuToggle>
            )}
          >
            <DropdownList>
              <DropdownItem
                onClick={() => {
                  setOpen(false);
                  onRemove();
                }}
              >
                Remove person…
              </DropdownItem>
            </DropdownList>
          </Dropdown>
        )}
      </Flex>
    </PageSection>
  );
}
