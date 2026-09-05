import { Label } from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import type { Service } from "../../runners/types";
import { statusText } from "../../runners/ui";
export function RunnerServiceStatus({ service }: { service: Service }) {
  return (
    <>
      {" "}
      <Label
        color={
          service.active === "failed"
            ? "red"
            : service.active === "active" && service.sub === "running"
              ? "green"
              : "grey"
        }
      >
        {statusText(service)}
      </Label>
      <CodeValue>
        {service.active}/{service.sub}; {service.enabled}
      </CodeValue>
    </>
  );
}
