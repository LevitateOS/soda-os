import type { Host, Release, Selection } from "./types";

const stable = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
export function availability(host: Host | null, release: Release | null) {
  const booted = host?.status.booted.image;
  if (!booted || !release) return { newer: false, message: "Check for an approved Soda release." };
  if (!stable.test(booted.version) || !stable.test(release.version)) {
    return {
      newer: false,
      message: "Development or unknown version: use native bootc for an explicit image selection.",
    };
  }
  const installed = booted.version.split(".").map(BigInt),
    available = release.version.split(".").map(BigInt);
  for (let index = 0; index < installed.length; index++) {
    if (installed[index] < available[index])
      return { newer: true, message: "A newer verified release is available." };
    if (installed[index] > available[index])
      return {
        newer: false,
        message:
          "This installation is newer than the latest published release. No downgrade will be offered.",
      };
  }
  return {
    newer: false,
    message:
      booted.imageDigest === release.reference.split("@")[1]
        ? "Up to date."
        : "Same version, different image. This may be a development candidate; it will not be replaced automatically.",
  };
}
export function stagedSelection(host: Host | null): Selection | null {
  const staged = host?.status.staged;
  if (!staged?.image || staged.incompatible) return null;
  return { version: staged.image.version, reference: staged.image.image.image };
}
