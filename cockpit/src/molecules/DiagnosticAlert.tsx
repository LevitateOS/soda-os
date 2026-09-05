import { Alert } from "@patternfly/react-core";
export function DiagnosticAlert({
  message,
  variant = "danger",
  role = "status",
}: {
  message: string;
  variant?: "danger" | "success" | "warning" | "info";
  role?: "status" | "alert";
}) {
  return (
    <Alert
      isInline
      variant={variant}
      title={message}
      className="soda-diagnostic"
      role={role}
      aria-live={role === "status" ? "polite" : undefined}
    />
  );
}
