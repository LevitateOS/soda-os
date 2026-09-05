import type { ReactNode } from "react";
export function ExternalLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="soda-external-link">
      {children}
    </a>
  );
}
