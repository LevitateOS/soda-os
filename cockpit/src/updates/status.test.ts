import { test, expect } from "vite-plus/test";
import { cachedUpdate, updateDiagnostic } from "./status";
import { hostFixture, nextDigest } from "../../tests/updates";

test("cached metadata belongs to its native source, including a staged source", () => {
  const host = hostFixture();
  const cached = { ...host.status.booted.image!, imageDigest: nextDigest };
  host.status.booted.cachedUpdate = cached;
  expect(cachedUpdate(host)).toBe(cached);
  host.spec.image = { image: "example.test/other:branch", transport: "registry" };
  expect(cachedUpdate(host)).toBeNull();
  const stagedCached = { ...cached, image: host.spec.image };
  host.status.staged = { ...host.status.booted, cachedUpdate: stagedCached };
  expect(cachedUpdate(host)).toBe(stagedCached);
  expect(updateDiagnostic(host)).toBeNull();
});

test("absent source and observations never establish eligibility or cached availability", () => {
  expect(cachedUpdate(null)).toBeNull();
  expect(updateDiagnostic(null)).toMatch(/unavailable/);
  const host = hostFixture();
  host.spec.image = null;
  expect(cachedUpdate(host)).toBeNull();
  expect(updateDiagnostic(host)).toMatch(/No native image source/);
  host.status.booted.image = null;
  expect(updateDiagnostic(host)).toMatch(/cannot manage/);
});
