// @vitest-environment jsdom
import { test, expect } from "vite-plus/test";
import { render, screen, within } from "@testing-library/react";
import { CockpitPageTemplate } from "./CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";

test("page composition keeps a named main, one heading, actions, and full native feedback", () => {
  const diagnostic = "Native operation failed:\n  git@example.test:team/repository.git";
  render(
    <CockpitPageTemplate
      title="Projects"
      description="Project catalog"
      busy
      actions={<button>Refresh</button>}
      feedback={<DiagnosticAlert message={diagnostic} />}
    >
      <section aria-label="Catalog">Catalog content</section>
    </CockpitPageTemplate>,
  );
  const main = screen.getByRole("main", { name: "Projects" });
  expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
  expect(within(main).getByRole("button", { name: "Refresh" })).toBeTruthy();
  expect(within(main).getByRole("region", { name: "Catalog" })).toBeTruthy();
  expect(within(main).getByRole("status").textContent).toContain(diagnostic);
  expect(main.closest('[aria-busy="true"]')).toBeTruthy();
});
