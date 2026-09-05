import type { ReactNode } from "react";
export function CodeValue({ children }: { children: ReactNode }) {
  return <code className="soda-code">{children}</code>;
}
