import { vi } from "vite-plus/test";
import type { Host, NativeUpdates } from "../src/updates/types";

export const source = "ghcr.io/levitateos/soda-os:dev-x86_64";
export const bootedDigest = "sha256:" + "a".repeat(64);
export const nextDigest = "sha256:" + "b".repeat(64);
export function hostFixture(): Host {
  const image = { image: source, transport: "registry" };
  return {
    apiVersion: "org.containers.bootc/v1",
    kind: "BootcHost",
    spec: { image },
    status: {
      booted: {
        image: { version: "0.6.3", imageDigest: bootedDigest, architecture: "amd64", image },
        cachedUpdate: null,
        downloadOnly: false,
        incompatible: false,
      },
      staged: null,
      rollbackQueued: false,
      usrOverlay: null,
      readOnly: false,
    },
  };
}
export function nativeFixture(host = hostFixture()) {
  return {
    status: vi.fn<NativeUpdates["status"]>().mockResolvedValue(host),
    check: vi.fn<NativeUpdates["check"]>().mockResolvedValue(host),
    update: vi.fn<NativeUpdates["update"]>().mockResolvedValue(undefined),
  };
}
