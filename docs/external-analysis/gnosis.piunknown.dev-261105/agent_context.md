# Repository Agent Context: `soda-os`

> This document is optimised for AI coding agents.
> For human-readable documentation see `onboarding.md`.

---

## System Overview

**Repository:** `soda-os` · https://github.com/levitateos/soda-os  
**Branch:** main  
**Files:** 291 analyzed of 291 discovered · Mode: Full  
**Languages:** Go (140 files), TypeScript (90 files), Markdown (33 files)  
**Functions:** 1084 · **Dependency edges:** 180 across 291 modules  
**Risk:** 2 CRITICAL · 118 HIGH · 17 MEDIUM · 94 LOW

## Critical Entry Points

Files not imported by any other file. Execution likely begins here.

- `src/pages/ProjectsPage.test.tsx` — imports 4 module(s) · risk: HIGH
- `src/pages/RunnersPage.test.tsx` — imports 4 module(s) · risk: MEDIUM
- `src/tailscale/native.test.ts` — imports 4 module(s) · risk: MEDIUM
- `src/pages/TailscalePage.test.tsx` — imports 2 module(s) · risk: LOW
- `src/pages/UpdatesPage.test.tsx` — imports 2 module(s) · risk: HIGH
- `src/projects/index.tsx` — imports 2 module(s) · risk: LOW
- `src/runners/index.tsx` — imports 2 module(s) · risk: LOW
- `src/tailscale/index.tsx` — imports 2 module(s) · risk: LOW
- `src/tailscale/status.test.ts` — imports 2 module(s) · risk: LOW
- `src/tailscale/stream.test.ts` — imports 2 module(s) · risk: LOW

## Core Components

Ranked by how many other files import them. High in-degree = high blast radius.

| Rank | File | In-degree | Risk | Avg CC |
|------|------|-----------|------|--------|
| 1 | `src/tailscale/types.ts` | 15 | LOW | 0.0 |
| 2 | `src/projects/types.ts` | 13 | LOW | 0.0 |
| 3 | `src/molecules/DiagnosticAlert.tsx` | 12 | LOW | 2.0 |
| 4 | `src/projects/ui.ts` | 9 | HIGH | 5.3 |
| 5 | `src/runners/types.ts` | 9 | LOW | 0.0 |
| 6 | `src/updates/types.ts` | 9 | LOW | 0.0 |
| 7 | `src/atoms/CodeValue.tsx` | 8 | LOW | 1.0 |
| 8 | `src/cockpit/types.ts` | 8 | LOW | 0.0 |
| 9 | `src/tailscale/status.ts` | 7 | HIGH | 4.7 |
| 10 | `src/atoms/ExternalLink.tsx` | 6 | LOW | 1.0 |

## Architectural Relationships

Direct relationships for the most-imported modules:

**`src/tailscale/types.ts`** — imported by 15 file(s)
  - Imported by: `src/pages/TailscalePage.test.tsx`, `src/tailscale/native.test.ts`, `src/tailscale/native.ts`, `src/tailscale/status.test.ts`, `src/tailscale/status.ts`, `src/tailscale/stream.test.ts` *(+9 more)*

**`src/projects/types.ts`** — imported by 13 file(s)
  - Imported by: `src/pages/ProjectsPage.test.tsx`, `src/pages/ProjectsPage.tsx`, `src/projects/native.ts`, `src/projects/protocol.ts`, `src/projects/ui.ts`, `src/projects/useProjects.ts` *(+7 more)*

**`src/molecules/DiagnosticAlert.tsx`** — imported by 12 file(s)
  - Imported by: `src/pages/ProjectsPage.tsx`, `src/pages/RunnersPage.tsx`, `src/pages/TailscalePage.tsx`, `src/templates/CockpitPageTemplate.test.tsx`, `molecules/updates/UpdateFeedback.tsx`, `organisms/projects/CatalogProjectDialog.tsx` *(+6 more)*

**`src/projects/ui.ts`** — imported by 9 file(s)
  - Imports: `src/projects/types.ts`
  - Imported by: `src/pages/ProjectsPage.tsx`, `src/projects/ui.test.ts`, `src/projects/useProjects.ts`, `molecules/projects/ProjectActions.tsx`, `molecules/projects/WorkspaceSummary.tsx`, `organisms/projects/CatalogProjectDialog.tsx` *(+3 more)*

**`src/runners/types.ts`** — imported by 9 file(s)
  - Imported by: `src/pages/RunnersPage.test.tsx`, `src/pages/RunnersPage.tsx`, `src/runners/native.ts`, `src/runners/protocol.ts`, `src/runners/ui.ts`, `src/runners/useRunners.ts` *(+3 more)*

**`src/updates/types.ts`** — imported by 9 file(s)
  - Imported by: `src/pages/UpdatesPage.test.tsx`, `src/pages/UpdatesPage.tsx`, `src/updates/native.ts`, `src/updates/status.ts`, `src/updates/useUpdates.ts`, `organisms/updates/ApplyUpdateDialog.tsx` *(+3 more)*

## High Risk Areas

**Do not modify without understanding the full dependency surface.**

| File | Risk | Max CC | Coupling | In-degree |
|------|------|--------|----------|-----------|
| `src/tailscale/useTailscale.ts` | CRITICAL | 36 | 3 | 1 |
| `src/runners/useRunners.ts` | CRITICAL | 28 | 2 | 1 |
| `src/projects/useProjects.ts` | HIGH | 19 | 2 | 1 |
| `src/updates/useUpdates.ts` | HIGH | 19 | 1 | 1 |
| `src/tailscale/stream.ts` | HIGH | 16 | 1 | 2 |
| `src/tailscale/native.ts` | HIGH | 14 | 3 | 2 |
| `organisms/tailscale/TailscaleConnection.tsx` | HIGH | 13 | 5 | 1 |
| `src/pages/ProjectsPage.tsx` | HIGH | 12 | 11 | 2 |
| `cockpit/tests/source-boundaries.test.ts` | HIGH | 11 | 0 | 0 |
| `internal/acceptance/qemu.go` | HIGH | 10 | 0 | 0 |
| `internal/acceptance/runner_vm.go` | HIGH | 10 | 0 | 0 |
| `internal/acceptance/scenarios.go` | HIGH | 10 | 0 | 0 |
| `internal/runners/native.go` | HIGH | 10 | 0 | 0 |
| `internal/updates/host.go` | HIGH | 10 | 0 | 0 |
| `internal/updates/operations.go` | HIGH | 10 | 0 | 0 |

## Circular Dependencies

No circular dependencies detected.

## Important Domain Objects

Classes ranked by the import importance of their containing file:

- **`nativeScriptFixture`** in `scripts/prepare_native_iso_candidate_test.go` — 0 method(s)
- **`releaseCommandState`** in `cmd/soda-image/main.go` — 0 method(s)
- **`commandRunner`** in `cmd/soda-release/main_test.go` — 0 method(s)
- **`statusRunner`** in `cmd/soda-updates/main_test.go` — 0 method(s)
- **`ArtifactSet`** in `internal/acceptance/artifacts.go` — 0 method(s)
- **`ValidatedArtifacts`** in `internal/acceptance/artifacts.go` — 0 method(s)
- **`CleanupAction`** in `internal/acceptance/cleanup.go` — 0 method(s)
- **`Cleanup`** in `internal/acceptance/cleanup.go` — 0 method(s)
- **`CommandSpec`** in `internal/acceptance/command.go` — 0 method(s)
- **`Secret`** in `internal/acceptance/evidence.go` — 0 method(s)
- **`Evidence`** in `internal/acceptance/evidence.go` — 0 method(s)
- **`bootcStatus`** in `internal/acceptance/fallback.go` — 0 method(s)
- **`forgejoKey`** in `internal/acceptance/identity_scenarios.go` — 0 method(s)
- **`scenarioIdentity`** in `internal/acceptance/identity_scenarios.go` — 0 method(s)
- **`ProcessSpec`** in `internal/acceptance/processes.go` — 0 method(s)

## Change Impact Guide

When modifying a file below, verify all direct dependents still behave correctly.

**If modifying `src/tailscale/types.ts` (LOW risk · 15 direct dependent(s)):**
  - `src/pages/TailscalePage.test.tsx`
  - `src/tailscale/native.test.ts`
  - `src/tailscale/native.ts`
  - `src/tailscale/status.test.ts`
  - `src/tailscale/status.ts`
  - `src/tailscale/stream.test.ts`
  - `src/tailscale/stream.ts`
  - *...and 8 more*

**If modifying `src/projects/types.ts` (LOW risk · 13 direct dependent(s)):**
  - `src/pages/ProjectsPage.test.tsx`
  - `src/pages/ProjectsPage.tsx`
  - `src/projects/native.ts`
  - `src/projects/protocol.ts`
  - `src/projects/ui.ts`
  - `src/projects/useProjects.ts`
  - `molecules/projects/CatalogFields.tsx`
  - *...and 6 more*

**If modifying `src/molecules/DiagnosticAlert.tsx` (LOW risk · 12 direct dependent(s)):**
  - `src/pages/ProjectsPage.tsx`
  - `src/pages/RunnersPage.tsx`
  - `src/pages/TailscalePage.tsx`
  - `src/templates/CockpitPageTemplate.test.tsx`
  - `molecules/updates/UpdateFeedback.tsx`
  - `organisms/projects/CatalogProjectDialog.tsx`
  - `organisms/projects/RemoveHumanDialog.tsx`
  - *...and 5 more*

**If modifying `src/projects/ui.ts` (HIGH risk · 9 direct dependent(s)):**
  - `src/pages/ProjectsPage.tsx`
  - `src/projects/ui.test.ts`
  - `src/projects/useProjects.ts`
  - `molecules/projects/ProjectActions.tsx`
  - `molecules/projects/WorkspaceSummary.tsx`
  - `organisms/projects/CatalogProjectDialog.tsx`
  - `organisms/projects/RemoveHumanDialog.tsx`
  - *...and 2 more*

**If modifying `src/runners/types.ts` (LOW risk · 9 direct dependent(s)):**
  - `src/pages/RunnersPage.test.tsx`
  - `src/pages/RunnersPage.tsx`
  - `src/runners/native.ts`
  - `src/runners/protocol.ts`
  - `src/runners/ui.ts`
  - `src/runners/useRunners.ts`
  - `molecules/runners/RunnerActions.tsx`
  - *...and 2 more*

**If modifying `src/updates/types.ts` (LOW risk · 9 direct dependent(s)):**
  - `src/pages/UpdatesPage.test.tsx`
  - `src/pages/UpdatesPage.tsx`
  - `src/updates/native.ts`
  - `src/updates/status.ts`
  - `src/updates/useUpdates.ts`
  - `organisms/updates/ApplyUpdateDialog.tsx`
  - `organisms/updates/AvailableReleaseSection.tsx`
  - *...and 2 more*

**If modifying `src/atoms/CodeValue.tsx` (LOW risk · 8 direct dependent(s)):**
  - `molecules/projects/WorkspaceSummary.tsx`
  - `molecules/runners/RunnerServiceStatus.tsx`
  - `organisms/projects/ProjectCatalog.tsx`
  - `organisms/runners/RunnerCapacity.tsx`
  - `organisms/updates/ApplyUpdateDialog.tsx`
  - `organisms/updates/AvailableReleaseSection.tsx`
  - `organisms/updates/InstalledImageSection.tsx`
  - *...and 1 more*

**If modifying `src/cockpit/types.ts` (LOW risk · 8 direct dependent(s)):**
  - `cockpit/tests/process.ts`
  - `src/projects/native.ts`
  - `src/runners/native.ts`
  - `src/tailscale/native.test.ts`
  - `src/tailscale/native.ts`
  - `src/tailscale/useTailscale.ts`
  - `src/updates/native.test.ts`
  - *...and 1 more*

## Safe Starting Points

Low complexity, low coupling. Read and modify these first.

- `cockpit/vite.config.ts` — avg CC: 0.0 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-forgejo-tailnet/main.go` — avg CC: 2.0 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-projects/main.go` — avg CC: 5.0 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-runner-helper/main.go` — avg CC: 5.0 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-runner-launch/main.go` — avg CC: 5.0 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-runners/main.go` — avg CC: 5.0 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-tailnet/main_test.go` — avg CC: 1.5 · coupling: 0 · imported by: 0 module(s)
- `cmd/soda-workspace-helper/main.go` — avg CC: 5.0 · coupling: 0 · imported by: 0 module(s)
- `cockpit/tests/build.test.ts` — avg CC: 0.0 · coupling: 0 · imported by: 0 module(s)
- `cockpit/tests/installed.test.ts` — avg CC: 1.0 · coupling: 0 · imported by: 0 module(s)

## AI Explanations

### `src/tailscale/types.ts` · LOW risk · in-degree: 15

This file, `cockpit/src/tailscale/types.ts`, is a TypeScript type definition file that exports a set of types used throughout the Tailscale-related components in the Cockpit application. Its primary responsibility is to define the shape of data that is used in these components, ensuring that the data conforms to a specific structure and is correctly typed.

In terms of architecture, this file sits at the top of the dependency chain, exporting types that are imported by various other files in the project, including `TailscalePage.test.tsx`, `native.test.ts`, and `useTailscale.ts`. This means that any changes made to the types defined in this file will have a ripple effect throughout the project, affecting the types used in these dependent files.

Key entry points for a new engineer to read first would be the exported types, such as `TailscaleStatus`, `TailscaleStream`, and `TailscaleError`. These types define the structure of the data used in the Tailscale components and should be understood before modifying any of the dependent files.

There are no significant risks or cautions to be aware of when modifying this file, as it has a low cyclomatic complexity and does not import any internal files. However, it is worth noting that any changes made to the types defined in this file will require corresponding changes to the dependent files that use these types.

### `src/projects/types.ts` · LOW risk · in-degree: 13

This file, cockpit/src/projects/types.ts, is responsible for defining type definitions for project-related data structures. It exports a set of TypeScript type interfaces, including Project, ProjectStatus, and CatalogField, which are used throughout the project to ensure type safety and consistency. These types are used to represent project metadata, such as project names, descriptions, and statuses, as well as catalog field definitions.

In terms of architecture, this file sits at the top of the dependency chain, providing type definitions that are imported by various other files in the project. It does not import any internal files, indicating that it is a self-contained module. The types defined in this file are used by multiple components, including the ProjectsPage, ProjectActions, and CatalogFields components, as well as several test files.

Key entry points for this file include the Project interface, which defines the shape of project data, and the CatalogField interface, which defines the shape of catalog field data. These interfaces are used extensively throughout the project and provide a clear understanding of the expected structure of project-related data.

There are no significant risks or cautions associated with modifying this file, as it is a relatively simple and self-contained module. However, it is worth noting that changes to this file may require corresponding changes to other files that import its types. A new engineer should read the Project interface first to understand the expected structure of project data.

### `src/molecules/DiagnosticAlert.tsx` · LOW risk · in-degree: 12

The `DiagnosticAlert` file is responsible for rendering a diagnostic alert component, which displays a message to the user. This component is a functional React component that takes in three props: `message`, `variant`, and `role`. The `message` prop is required and represents the text to be displayed in the alert. The `variant` prop is optional and determines the color scheme of the alert, with options including "danger", "success", "warning", and "info". The `role` prop is also optional and determines the accessibility role of the alert, with options including "status" and "alert".

In terms of architecture, the `DiagnosticAlert` file is a leaf component in the dependency chain, meaning it does not import any internal files and is instead imported by 12 other files in the project. This suggests that the component is a reusable piece of code that can be easily integrated into various parts of the application.

Key entry points for this file include the `DiagnosticAlert` function, which is the main export of the file. This function is responsible for rendering the alert component and takes in the three props mentioned earlier. There are no risks or cautions to be aware of when modifying this file, as it has a low cyclomatic complexity and does not exhibit any circular dependencies or high coupling. However, it is worth noting that the component does not have any docstrings, which may make it more difficult for other engineers to understand its purpose and usage.

A new engineer should read the `DiagnosticAlert` function first to understand how the component is rendered and how it can be customized with different props.

### `src/projects/ui.ts` · HIGH risk · in-degree: 9

This file, cockpit/src/projects/ui.ts, is responsible for handling user interface logic for project-related operations. It exports seven functions: payloadFor, catalogPayload, successMessage, errorMessage, sshCommand, humanDeletionHidden, and projectRemovalHidden. These functions are used to generate payloads for API requests, display success and error messages, construct SSH commands, and determine whether human or project removal is hidden based on the current user's role.

In terms of architecture, this file imports types from ./types and is imported by nine other files, including ProjectsPage.tsx, ui.test.ts, and useProjects.ts. It has a high coupling with the internal file types.ts and a high cyclomatic complexity, with the payloadFor function having a maximum complexity of 10.

Key entry points for a new engineer should be the payloadFor function, which is the primary entry point for generating payloads for API requests, and the catalogPayload function, which is used to construct payloads for catalog-related operations. The successMessage and errorMessage functions are also important, as they are used to display messages to the user.

Before modifying this file, a new engineer should be aware of the high cyclomatic complexity of the payloadFor function and the lack of docstrings for all seven functions. They should also be cautious when modifying the catalogPayload function, as it has a try-catch block that may throw errors if the additional metadata is not a valid JSON object. Additionally, the humanDeletionHidden and projectRemovalHidden functions should be modified with care, as they affect the visibility of human and project removal operations based on the current user's role. A new engineer should read the payloadFor function first to understand the overall logic of the file.

### `src/runners/types.ts` · LOW risk · in-degree: 9

This file, `cockpit/src/runners/types.ts`, defines type definitions for the runners module. Its primary responsibility is to provide type annotations for the runners-related data structures and functions, ensuring consistency and correctness throughout the codebase. The types defined in this file are used to represent runner configurations, statuses, and actions, which are essential for the runners module to function properly.

In terms of architecture, this file sits at the top of the dependency chain for the runners module. It is imported by several files, including `RunnersPage.test.tsx`, `RunnersPage.tsx`, and `RunnerServiceStatus.tsx`, which rely on the type definitions provided by this file to access and manipulate runner-related data. The types defined in this file are also used by the `RunnerActions` class, which is responsible for handling runner-related actions.

Key entry points for a new engineer to read first include the `RunnerConfig` and `RunnerStatus` types, which define the structure of runner configurations and statuses, respectively. These types are used extensively throughout the runners module and are essential for understanding how the module works. Additionally, the `RunnerAction` type is also worth reading, as it defines the possible actions that can be performed on a runner.

There are no significant risks or cautions associated with modifying this file, as it only contains type definitions and does not contain any complex logic or dependencies. However, it is essential to ensure that any changes to the types defined in this file do not break the existing functionality of the runners module. A new engineer should start by reading the `RunnerConfig` type to understand the structure of runner configurations.

### `src/updates/types.ts` · LOW risk · in-degree: 9

This file, cockpit/src/updates/types.ts, defines type definitions for the updates feature in the Cockpit application. Its primary responsibility is to provide type annotations for the updates-related data structures, such as the Update and Release types. These types are used throughout the application to ensure consistency and correctness in handling updates.

In terms of architecture, this file sits at the top of the dependency chain, providing type definitions that are imported by various other files in the application. It does not import any internal files, indicating a low coupling with other parts of the codebase. This file is a dependency for 9 other files, including the UpdatesPage component and its test file, as well as several utility functions.

Key entry points for this file include the Update and Release types, which are defined using TypeScript's type syntax. The Update type has properties such as id, name, and version, while the Release type has properties such as id, name, and description. These types are used to represent the data associated with updates and releases in the application.

There are no significant risks or cautions associated with modifying this file, given its low complexity and lack of dependencies. However, it is worth noting that this file does not contain any docstrings or comments, which may make it more difficult for new engineers to understand the context and purpose of the type definitions. To get started with this file, a new engineer should read the definitions of the Update and Release types to understand the structure and properties of the data used in the updates feature.

### `src/atoms/CodeValue.tsx` · LOW risk · in-degree: 8

The `CodeValue` file is responsible for rendering code snippets within the Cockpit application. It exports a single function, `CodeValue`, which takes a `children` prop of type `ReactNode`. This function returns a `<code>` element with a class name of "soda-code", containing the provided `children` as its content. The primary purpose of this file is to provide a standardized way of displaying code snippets throughout the application.

In terms of architecture, the `CodeValue` file is a low-level component that is imported by multiple other files in the Cockpit codebase, including `WorkspaceSummary.tsx`, `RunnerServiceStatus.tsx`, and `ProjectCatalog.tsx`. It does not import any internal files, indicating a low coupling with other components. The average cyclomatic complexity of this file is 1.0, suggesting a simple and straightforward implementation.

There are no specific risks or cautions to be aware of when modifying this file, as it has a low cyclomatic complexity and does not exhibit any circular dependencies. However, it is worth noting that the file does not contain any docstrings, which may make it more difficult for new engineers to understand the purpose and behavior of the `CodeValue` function without additional context. To get started with this file, a new engineer should first read the `CodeValue` function definition to understand its purpose and behavior.

### `src/cockpit/types.ts` · LOW risk · in-degree: 8

This file, cockpit/src/cockpit/types.ts, is a TypeScript file that defines type definitions for the Cockpit project. Its primary responsibility is to provide type annotations for various types, interfaces, and enums used throughout the project. This file serves as a centralized location for type definitions, ensuring consistency and accuracy across the codebase.

In terms of architecture, this file is a dependency of eight other files, including process.ts, native.ts, and useTailscale.ts. It does not import any internal files, indicating a low coupling with other parts of the codebase. This file's position in the dependency chain suggests that it provides foundational type definitions that are used by other components.

Key entry points for this file include the various type definitions, such as the `CockpitConfig` interface and the `CockpitError` enum. These definitions are likely used throughout the project to ensure type safety and consistency. A new engineer should read the `CockpitConfig` interface first, as it provides a clear understanding of the expected configuration for the Cockpit project.

There are no significant risks or cautions associated with modifying this file, given its low cyclomatic complexity and lack of dependencies. However, it is essential to note that this file is a dependency of multiple other files, so any changes made here may have a ripple effect throughout the codebase.

### `src/tailscale/status.ts` · HIGH risk · in-degree: 7

This file, `cockpit/src/tailscale/status.ts`, is responsible for managing the Tailscale connection status and related data. It exports seven functions that handle various aspects of the connection, including determining the connection state, selecting an exit node, and checking for exit node approval. The file also imports types and data from `./types` and `./types.ts`.

In terms of architecture, this file sits at the center of the Tailscale connection management, importing data from `./types` and being imported by seven other files, including `status.test.ts`, `useTailscale.ts`, and `TailscaleDevices.tsx`. This suggests that it plays a crucial role in the overall Tailscale connection management system.

Key entry points for a new engineer include the `connectionState` function, which determines the current connection state based on the `Status` object, and the `exitSelection` function, which selects an exit node based on the `Snapshot` object. The `exitNodeApproval` function is also an important entry point, as it checks the approval status of the exit node.

However, there are risks and cautions that a new engineer should be aware of before modifying this file. The high cyclomatic complexity of the `connectionState` function (max 10) and the lack of docstrings for all functions (100% of 7 functions have no docstring) make it a challenging file to modify. Additionally, the file imports from only one internal file (`./types.ts`) and is imported by seven other files, which may indicate a high coupling risk. A new engineer should carefully review the code and consider these risks before making any changes. To start, they should read the `connectionState` function to understand the connection state logic.

### `src/atoms/ExternalLink.tsx` · LOW risk · in-degree: 6

The `ExternalLink` file is responsible for rendering an external link in the application. It exports a single function, `ExternalLink`, which takes two props: `href` and `children`. The `href` prop specifies the URL of the external link, while the `children` prop contains the text or ReactNode to be displayed as the link's content. The function returns an `a` HTML element with the specified `href`, `target`, and `rel` attributes, as well as a `className` attribute set to "soda-external-link". This allows the link to be styled consistently throughout the application.

In terms of architecture, the `ExternalLink` file is a low-level component that is imported by six other files in the codebase, including `TailscalePage.tsx` and `ForgejoRegistrationFields.tsx`. It does not import any internal files, indicating a low coupling with other components. The file has a low cyclomatic complexity of 1.0, making it a straightforward and easy-to-understand piece of code.

Before modifying this file, it is essential to note that the `ExternalLink` function does not include any error handling or validation for the `href` prop. This means that if an invalid or malformed URL is passed to the function, it may not render correctly or may cause unexpected behavior. Additionally, the file does not include any docstrings or comments, which may make it more challenging for new engineers to understand the purpose and behavior of the component. To get started with this file, a new engineer should read the `ExternalLink` function definition to understand its props and behavior.

### `src/runners/ui.ts` · HIGH risk · in-degree: 5

This file, `cockpit/src/runners/ui.ts`, is responsible for generating UI-related data structures and utility functions for the runners module. It exports eight functions that handle tasks such as creating payload data for registration, generating status text and class for services, and constructing URLs for providers.

In terms of architecture, this file imports types from `./types.ts` and is imported by five other files, indicating its role as a utility module that provides functionality to other parts of the codebase. The high cyclomatic complexity of the `createPayload` function, with a maximum of 10 conditional statements, is a risk that should be addressed through refactoring.

Key entry points for a new engineer include the `createPayload` function, which is responsible for generating registration data, and the `statusText` and `statusClass` functions, which determine the status of services. The `forgejoBrowserURL` function is also noteworthy, as it constructs URLs for the Forgejo provider.

Before modifying this file, a new engineer should be aware of the high risk of circular dependencies and the lack of docstrings for 100% of the functions, which can make it difficult to understand the purpose and behavior of each function. Additionally, the high cyclomatic complexity of the `createPayload` function makes it a potential source of errors. To get started, a new engineer should read the `createPayload` function first to understand how it generates registration data.

### `src/templates/CockpitPageTemplate.tsx` · LOW risk · in-degree: 5

This file, CockpitPageTemplate.tsx, is responsible for rendering the main content of the Cockpit page. It takes in various props such as title, description, actions, feedback, children, dialogs, and busy state, and uses these to construct the page layout. The file primarily uses components from the @patternfly/react-core library, including Page, PageSection, Stack, and StackItem, to create a structured page with a sidebar, main content, and optional feedback and dialogs.

In terms of architecture, this file imports the PageHeading component from the ../molecules/PageHeading file and is itself imported by five other files, including ProjectsPage.tsx, RunnersPage.tsx, and CockpitPageTemplate.test.tsx. This suggests that CockpitPageTemplate.tsx is a key component in the Cockpit page's rendering pipeline, and its output is likely used as input by other components.

New engineers should focus on understanding the CockpitPageTemplate function, which is the primary entry point of this file. This function takes in the various props and returns the JSX structure of the page. They should also familiarize themselves with the PageHeading component, which is used to render the page's title, description, and actions.

One risk to be aware of is that this file has a relatively low cyclomatic complexity, but it does import components from other files, which could introduce coupling issues if not managed carefully. Additionally, the file does not have any docstrings, which may make it more difficult for new engineers to understand its purpose and usage without additional context. To get started, new engineers should read the CockpitPageTemplate function and its props to understand how it constructs the page layout.

### `src/pages/ProjectsPage.tsx` · HIGH risk · in-degree: 2

The `ProjectsPage` file is responsible for rendering the projects page in the Cockpit application. It is a React functional component that imports various components and hooks from other files to display a catalog of projects, allow users to add or remove projects, and manage workspace setup. The primary responsibility of this file is to orchestrate the display of projects and handle user interactions.

In terms of architecture, this file sits at the top of the dependency chain, importing components and hooks from other files such as `CockpitPageTemplate`, `DiagnosticAlert`, `ProjectCatalog`, and `useProjects`. It is also imported by two other files, `ProjectsPage.test.tsx` and `index.tsx`, indicating its central role in the application.

Key entry points for a new engineer should be the `ProjectsPage` function and the `useProjects` hook. The `ProjectsPage` function is the main entry point, and understanding its structure and logic is crucial to modifying this file. The `useProjects` hook is also essential, as it provides the data and functionality for the projects page.

There are several risks and cautions that a new engineer should be aware of before modifying this file. The high cyclomatic complexity of the `ProjectsPage` function (12) and the average complexity across the file (12.0) indicate a high risk of errors or bugs. Additionally, the file imports components and hooks from 11 internal files, which can make it difficult to understand the dependencies and relationships between components. Furthermore, the lack of docstrings or comments in some functions and classes may make it challenging to understand their purpose and behavior.

### `src/pages/RunnersPage.tsx` · HIGH risk · in-degree: 2

The RunnersPage file is responsible for rendering the Runners page, which allows users to create and manage local capacity for provider-owned CI workflows. This file is a React functional component that imports various internal modules to display a complex UI. It uses the useRunners hook to fetch data and manage state, and it renders several child components, including RunnerCapacity, RunnerExecutionNotice, and ProviderAuthoritySection.

In terms of architecture, this file sits at the top of the dependency chain, importing 9 unique internal modules and being imported by 2 other files. Its high coupling and cyclomatic complexity (5.0) indicate that it is a central component that integrates multiple functionalities.

New engineers should start by reading the RunnersPage function, which is the main entry point of this file. They should also examine the useRunners hook, which is responsible for fetching data and managing state. Additionally, they should review the child components, such as RunnerCapacity and RunnerExecutionNotice, to understand how they are used and integrated into the overall UI.

Before modifying this file, engineers should be aware of the following risks: the high coupling and cyclomatic complexity, which can make it difficult to understand and modify; the 9 unique internal module imports, which can lead to tight coupling and make it harder to refactor; and the lack of docstrings, which can make it harder to understand the code. Engineers should also be cautious when modifying the useRunners hook, as it is responsible for fetching data and managing state, and changes to it can have far-reaching consequences.

### `src/pages/TailscalePage.tsx` · HIGH risk · in-degree: 2

The `TailscalePage` file is responsible for rendering the Tailscale page in the Cockpit application. It imports various components from internal modules and uses the `useTailscale` hook to fetch data from the Tailscale API. The file exports a single function, `TailscalePage`, which takes `TailscaleOptions` as a prop and returns a JSX element that represents the Tailscale page.

In terms of architecture, `TailscalePage` sits at the top of the dependency chain, importing components from various internal modules, including `CockpitPageTemplate`, `DiagnosticAlert`, `ExternalLink`, and others. It is also imported by two other files, `TailscalePage.test.tsx` and `index.tsx`.

Key entry points for a new engineer include the `TailscalePage` function itself, as well as the `useTailscale` hook, which is used to fetch data from the Tailscale API. The `CockpitPageTemplate` component is also an important entry point, as it provides the basic structure for the page.

There are several risks and cautions that a new engineer should be aware of when modifying this file. The high coupling between this file and other internal modules makes it difficult to modify without affecting other parts of the application. Additionally, the file has a high cyclomatic complexity, with a maximum complexity of 2 in the `TailscalePage` function. This suggests that the code may be difficult to understand and modify. Finally, the file has no docstrings, which makes it difficult for new engineers to understand the purpose and behavior of the code.

---

**Generated by Project Gnosis (Code Archaeology Agent)**  
**Repository:** https://github.com/levitateos/soda-os  
**Branch:** main  
**Generated:** 2026-09-05 15:24 UTC  
**Analysis Mode:** Full
