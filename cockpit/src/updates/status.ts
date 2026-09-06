import type { Host } from "./types";

export function updateDiagnostic(host: Host | null): string | null {
  if (!host) return "Native deployment status is unavailable. Refresh status before updating.";
  if (host.status.rollbackQueued)
    return "A rollback is queued. Resolve it with native bootc before updating.";
  if (host.status.usrOverlay) return "A /usr overlay is active. Resolve it before updating.";
  if (host.status.readOnly) return "The bootc system is read-only; updating is unavailable.";
  if (host.status.booted.incompatible || !host.status.booted.image)
    return "bootc cannot manage the current deployment.";
  if (host.status.staged?.incompatible)
    return "bootc cannot manage the staged deployment. Inspect native status before updating.";
  if (!host.spec.image) return "No native image source is configured. Inspect bootc status.";
  return null;
}

export function cachedUpdate(host: Host | null) {
  const source = host?.spec.image;
  if (!source) return null;
  for (const entry of [host.status.staged, host.status.booted]) {
    const cached = entry?.cachedUpdate;
    if (cached?.image.image === source.image && cached.image.transport === source.transport)
      return cached;
  }
  return null;
}
