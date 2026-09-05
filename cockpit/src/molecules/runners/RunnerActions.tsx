import { Button, Flex } from "@patternfly/react-core";
import type { Runner, LifecycleAction } from "../../runners/types";
export function RunnerActions({
  runner,
  busy,
  onAction,
  onRemove,
}: {
  runner: Runner;
  busy: boolean;
  onAction: (action: LifecycleAction, id: string) => void;
  onRemove: (id: string) => void;
}) {
  return (
    <Flex gap={{ default: "gapSm" }}>
      <Button
        size="sm"
        variant={runner.service.active === "active" ? "secondary" : "primary"}
        isDisabled={busy}
        onClick={() => onAction(runner.service.active === "active" ? "stop" : "start", runner.id)}
      >
        {runner.service.active === "active" ? "Stop" : "Start"}
      </Button>
      <Button
        size="sm"
        variant="secondary"
        isDisabled={busy}
        onClick={() => onAction("restart", runner.id)}
      >
        Restart
      </Button>
      <Button
        size="sm"
        variant="link"
        isDanger
        isDisabled={busy}
        onClick={() => onRemove(runner.id)}
      >
        Remove
      </Button>
    </Flex>
  );
}
