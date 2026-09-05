# soda-os — Architecture Overview

## Project Summary

**soda-os** contains **291** analyzed files (291 discovered).

**Languages:** Go (140 files), TypeScript (90 files), Markdown (33 files), TOML (21 files), YAML (6 files)  
**Total functions extracted:** 1084  
**Risk distribution:** 2 CRITICAL · 118 HIGH · 17 MEDIUM · 94 LOW

## Repository Statistics

| Metric | Value |
|--------|-------|
| Files discovered | 291 |
| Files analyzed | 291 |
| Analysis mode | Full |
| Primary language | Go (48%) |
| Total functions | 1084 |
| Total classes | 193 |
| Dependency graph nodes | 291 |
| Dependency graph edges | 180 |
| Circular dependencies | 0 |
| CRITICAL risk files | 2 |
| HIGH risk files | 118 |

## Architecture Map

Top files by dependency in-degree (how many other files import each one):

| File Path | In-Degree (Imports) | Risk Level |
| :--- | :---: | :---: |
| `src/tailscale/types.ts` | 15 | **LOW** |
| `src/projects/types.ts` | 13 | **LOW** |
| `src/molecules/DiagnosticAlert.tsx` | 12 | **LOW** |
| `src/projects/ui.ts` | 9 | **HIGH** |
| `src/runners/types.ts` | 9 | **LOW** |
| `src/updates/types.ts` | 9 | **LOW** |
| `src/atoms/CodeValue.tsx` | 8 | **LOW** |
| `src/cockpit/types.ts` | 8 | **LOW** |
| `src/tailscale/status.ts` | 7 | **HIGH** |
| `src/atoms/ExternalLink.tsx` | 6 | **LOW** |
| `src/runners/ui.ts` | 5 | **HIGH** |
| `src/templates/CockpitPageTemplate.tsx` | 5 | **LOW** |
| `organisms/projects/dialogTypes.ts` | 4 | **LOW** |
| `cockpit/tests/process.ts` | 3 | **LOW** |
| `src/molecules/ConfirmationField.tsx` | 3 | **LOW** |

## Core Components

### `src/runners/useRunners.ts`
**Risk:** CRITICAL · **In-degree:** 1 · **Avg CC:** 28.0
> **Depends on:** src/runners/types.ts, src/runners/ui.ts  
> **Depended on by:** src/pages/RunnersPage.tsx

This file, `useRunners.ts`, is responsible for managing the state and behavior of a list of runners in the cockpit application. It provides a set of functions and state variables that can be used to interact with the runners, including loading, creating, and removing them. *(full explanation in File Explanations document)*

### `src/tailscale/useTailscale.ts`
**Risk:** CRITICAL · **In-degree:** 1 · **Avg CC:** 14.3
> **Depends on:** src/cockpit/types.ts, src/tailscale/types.ts, src/tailscale/status.ts  
> **Depended on by:** src/pages/TailscalePage.tsx

This file, `useTailscale.ts`, is responsible for managing the Tailscale connection and providing the necessary data to the application. It uses the `native` object to interact with the Tailscale API and updates the application state accordingly. *(full explanation in File Explanations document)*

### `src/tailscale/types.ts`
**Risk:** LOW · **In-degree:** 15 · **Avg CC:** 0.0
> **Depended on by:** src/pages/TailscalePage.test.tsx, src/tailscale/native.test.ts, src/tailscale/native.ts, src/tailscale/status.test.ts, src/tailscale/status.ts, src/tailscale/stream.test.ts

This file, `cockpit/src/tailscale/types.ts`, is a TypeScript type definition file that exports a set of types used throughout the Tailscale-related components in the Cockpit application. Its primary responsibility is to define the shape of data that is used in these components, ensuring that the data conforms to a specific structure and is correctly typed. *(full explanation in File Explanations document)*

### `src/projects/types.ts`
**Risk:** LOW · **In-degree:** 13 · **Avg CC:** 0.0
> **Depended on by:** src/pages/ProjectsPage.test.tsx, src/pages/ProjectsPage.tsx, src/projects/native.ts, src/projects/protocol.ts, src/projects/ui.ts, src/projects/useProjects.ts

This file, cockpit/src/projects/types.ts, is responsible for defining type definitions for project-related data structures. It exports a set of TypeScript type interfaces, including Project, ProjectStatus, and CatalogField, which are used throughout the project to ensure type safety and consistency. *(full explanation in File Explanations document)*

### `src/molecules/DiagnosticAlert.tsx`
**Risk:** LOW · **In-degree:** 12 · **Avg CC:** 2.0
> **Depended on by:** src/pages/ProjectsPage.tsx, src/pages/RunnersPage.tsx, src/pages/TailscalePage.tsx, src/templates/CockpitPageTemplate.test.tsx, molecules/updates/UpdateFeedback.tsx, organisms/projects/CatalogProjectDialog.tsx

The `DiagnosticAlert` file is responsible for rendering a diagnostic alert component, which displays a message to the user. This component is a functional React component that takes in three props: `message`, `variant`, and `role`. *(full explanation in File Explanations document)*

### `src/projects/ui.ts`
**Risk:** HIGH · **In-degree:** 9 · **Avg CC:** 5.3
> **Depends on:** src/projects/types.ts  
> **Depended on by:** src/pages/ProjectsPage.tsx, src/projects/ui.test.ts, src/projects/useProjects.ts, molecules/projects/ProjectActions.tsx, molecules/projects/WorkspaceSummary.tsx, organisms/projects/CatalogProjectDialog.tsx

This file, cockpit/src/projects/ui.ts, is responsible for handling user interface logic for project-related operations. It exports seven functions: payloadFor, catalogPayload, successMessage, errorMessage, sshCommand, humanDeletionHidden, and projectRemovalHidden. *(full explanation in File Explanations document)*

### `src/runners/types.ts`
**Risk:** LOW · **In-degree:** 9 · **Avg CC:** 0.0
> **Depended on by:** src/pages/RunnersPage.test.tsx, src/pages/RunnersPage.tsx, src/runners/native.ts, src/runners/protocol.ts, src/runners/ui.ts, src/runners/useRunners.ts

This file, `cockpit/src/runners/types.ts`, defines type definitions for the runners module. Its primary responsibility is to provide type annotations for the runners-related data structures and functions, ensuring consistency and correctness throughout the codebase. *(full explanation in File Explanations document)*

### `src/updates/types.ts`
**Risk:** LOW · **In-degree:** 9 · **Avg CC:** 0.0
> **Depended on by:** src/pages/UpdatesPage.test.tsx, src/pages/UpdatesPage.tsx, src/updates/native.ts, src/updates/status.ts, src/updates/useUpdates.ts, organisms/updates/ApplyUpdateDialog.tsx

This file, cockpit/src/updates/types.ts, defines type definitions for the updates feature in the Cockpit application. Its primary responsibility is to provide type annotations for the updates-related data structures, such as the Update and Release types. *(full explanation in File Explanations document)*

### `src/atoms/CodeValue.tsx`
**Risk:** LOW · **In-degree:** 8 · **Avg CC:** 1.0
> **Depended on by:** molecules/projects/WorkspaceSummary.tsx, molecules/runners/RunnerServiceStatus.tsx, organisms/projects/ProjectCatalog.tsx, organisms/runners/RunnerCapacity.tsx, organisms/updates/ApplyUpdateDialog.tsx, organisms/updates/AvailableReleaseSection.tsx

The `CodeValue` file is responsible for rendering code snippets within the Cockpit application. It exports a single function, `CodeValue`, which takes a `children` prop of type `ReactNode`. *(full explanation in File Explanations document)*

### `src/cockpit/types.ts`
**Risk:** LOW · **In-degree:** 8 · **Avg CC:** 0.0
> **Depended on by:** cockpit/tests/process.ts, src/projects/native.ts, src/runners/native.ts, src/tailscale/native.test.ts, src/tailscale/native.ts, src/tailscale/useTailscale.ts

This file, cockpit/src/cockpit/types.ts, is a TypeScript file that defines type definitions for the Cockpit project. Its primary responsibility is to provide type annotations for various types, interfaces, and enums used throughout the project. *(full explanation in File Explanations document)*

### `src/tailscale/status.ts`
**Risk:** HIGH · **In-degree:** 7 · **Avg CC:** 4.7
> **Depends on:** src/tailscale/types.ts  
> **Depended on by:** src/tailscale/status.test.ts, src/tailscale/useTailscale.ts, molecules/tailscale/DeviceIdentity.tsx, molecules/tailscale/ExitNodeApprovalGuidance.tsx, organisms/tailscale/ExitNodeForm.tsx, organisms/tailscale/TailscaleConnection.tsx

This file, `cockpit/src/tailscale/status.ts`, is responsible for managing the Tailscale connection status and related data. It exports seven functions that handle various aspects of the connection, including determining the connection state, selecting an exit node, and checking for exit node approval. *(full explanation in File Explanations document)*

### `src/atoms/ExternalLink.tsx`
**Risk:** LOW · **In-degree:** 6 · **Avg CC:** 1.0
> **Depended on by:** src/pages/TailscalePage.tsx, molecules/runners/ForgejoRegistrationFields.tsx, molecules/tailscale/AuthenticationGuidance.tsx, molecules/tailscale/ExitNodeApprovalGuidance.tsx, organisms/runners/RunnerCapacity.tsx, organisms/updates/AvailableReleaseSection.tsx

The `ExternalLink` file is responsible for rendering an external link in the application. It exports a single function, `ExternalLink`, which takes two props: `href` and `children`. *(full explanation in File Explanations document)*

### `src/runners/ui.ts`
**Risk:** HIGH · **In-degree:** 5 · **Avg CC:** 4.6
> **Depends on:** src/runners/types.ts  
> **Depended on by:** src/runners/ui.test.ts, src/runners/useRunners.ts, molecules/runners/ForgejoRegistrationFields.tsx, molecules/runners/RunnerServiceStatus.tsx, organisms/runners/RunnerCapacity.tsx

This file, `cockpit/src/runners/ui.ts`, is responsible for generating UI-related data structures and utility functions for the runners module. It exports eight functions that handle tasks such as creating payload data for registration, generating status text and class for services, and constructing URLs for providers. *(full explanation in File Explanations document)*

### `src/templates/CockpitPageTemplate.tsx`
**Risk:** LOW · **In-degree:** 5 · **Avg CC:** 2.0
> **Depends on:** src/molecules/PageHeading.tsx  
> **Depended on by:** src/pages/ProjectsPage.tsx, src/pages/RunnersPage.tsx, src/pages/TailscalePage.tsx, src/pages/UpdatesPage.tsx, src/templates/CockpitPageTemplate.test.tsx

This file, CockpitPageTemplate.tsx, is responsible for rendering the main content of the Cockpit page. It takes in various props such as title, description, actions, feedback, children, dialogs, and busy state, and uses these to construct the page layout. *(full explanation in File Explanations document)*

### `src/pages/ProjectsPage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 12.0
> **Depends on:** src/templates/CockpitPageTemplate.tsx, src/molecules/DiagnosticAlert.tsx, organisms/projects/ProjectCatalog.tsx, organisms/projects/PeopleSection.tsx, organisms/projects/CatalogProjectDialog.tsx, organisms/projects/WorkspaceSetupDialog.tsx  
> **Depended on by:** src/pages/ProjectsPage.test.tsx, src/projects/index.tsx

The `ProjectsPage` file is responsible for rendering the projects page in the Cockpit application. It is a React functional component that imports various components and hooks from other files to display a catalog of projects, allow users to add or remove projects, and manage workspace setup. *(full explanation in File Explanations document)*

### `src/pages/RunnersPage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 5.0
> **Depends on:** src/templates/CockpitPageTemplate.tsx, src/molecules/DiagnosticAlert.tsx, organisms/runners/RunnerCapacity.tsx, organisms/runners/RunnerExecutionNotice.tsx, organisms/runners/ProviderAuthoritySection.tsx, organisms/runners/RegisterRunnerDialog.tsx  
> **Depended on by:** src/pages/RunnersPage.test.tsx, src/runners/index.tsx

The RunnersPage file is responsible for rendering the Runners page, which allows users to create and manage local capacity for provider-owned CI workflows. This file is a React functional component that imports various internal modules to display a complex UI. *(full explanation in File Explanations document)*

### `src/pages/TailscalePage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 2.0
> **Depends on:** src/templates/CockpitPageTemplate.tsx, src/molecules/DiagnosticAlert.tsx, src/atoms/ExternalLink.tsx, organisms/tailscale/TailscaleConnection.tsx, organisms/tailscale/TailscaleDevices.tsx, organisms/tailscale/ExitNodeForm.tsx  
> **Depended on by:** src/pages/TailscalePage.test.tsx, src/tailscale/index.tsx

The `TailscalePage` file is responsible for rendering the Tailscale page in the Cockpit application. It imports various components from internal modules and uses the `useTailscale` hook to fetch data from the Tailscale API. *(full explanation in File Explanations document)*

### `src/pages/UpdatesPage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 4.0
> **Depends on:** src/templates/CockpitPageTemplate.tsx, molecules/updates/UpdateFeedback.tsx, molecules/updates/NativeOperationOutput.tsx, organisms/updates/InstalledImageSection.tsx, organisms/updates/AvailableReleaseSection.tsx, organisms/updates/PendingDeploymentSection.tsx  
> **Depended on by:** src/pages/UpdatesPage.test.tsx, src/updates/index.tsx

The UpdatesPage file is responsible for rendering the updates page in the cockpit application. It imports various components and utilities from internal modules and uses them to display the current updates status, available releases, and pending deployments. *(full explanation in File Explanations document)*

### `src/projects/protocol.ts`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 3.6
> **Depends on:** src/projects/types.ts  
> **Depended on by:** src/projects/native.ts, src/projects/protocol.test.ts

This file, `protocol.ts`, is responsible for encoding and decoding messages between the cockpit application and a coordinator process. It provides functions to assert the correctness of coordinator responses and to construct requests to the coordinator. *(full explanation in File Explanations document)*

### `src/runners/protocol.ts`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 4.7
> **Depends on:** src/runners/types.ts  
> **Depended on by:** src/runners/native.ts, src/runners/protocol.test.ts

This file, `protocol.ts`, is responsible for encoding and decoding messages exchanged between the cockpit and runner coordinator. It provides functions to validate and transform data according to the protocol's rules. *(full explanation in File Explanations document)*

### `organisms/projects/dialogTypes.ts`
**Risk:** LOW · **In-degree:** 4 · **Avg CC:** 0.0
> **Depended on by:** organisms/projects/CatalogProjectDialog.tsx, organisms/projects/RemoveHumanDialog.tsx, organisms/projects/RemoveProjectDialog.tsx, organisms/projects/WorkspaceSetupDialog.tsx

### `cockpit/tests/process.ts`
**Risk:** LOW · **In-degree:** 3 · **Avg CC:** 1.0
> **Depends on:** src/cockpit/types.ts  
> **Depended on by:** src/pages/ProjectsPage.test.tsx, src/pages/RunnersPage.test.tsx, src/tailscale/native.test.ts

### `src/molecules/ConfirmationField.tsx`
**Risk:** LOW · **In-degree:** 3 · **Avg CC:** 1.0
> **Depended on by:** organisms/projects/RemoveHumanDialog.tsx, organisms/projects/RemoveProjectDialog.tsx, organisms/runners/RemoveRunnerDialog.tsx

## Tech Debt Report

### Circular Dependencies

No circular dependencies detected.

### CRITICAL Risk Files

| File | Max CC | Avg CC | Coupling | Notes |
|------|--------|--------|----------|-------|
| `src/tailscale/useTailscale.ts` | 36 | 14.3 | 3 | CC=36 |
| `src/runners/useRunners.ts` | 28 | 28.0 | 2 | CC=28 |

### High Complexity Functions

| File | Function | Cyclomatic Complexity |
|------|----------|-----------------------|
| `src/tailscale/useTailscale.ts` | `useTailscale` | 36 |
| `src/runners/useRunners.ts` | `useRunners` | 28 |
| `src/projects/useProjects.ts` | `useProjects` | 19 |
| `src/updates/useUpdates.ts` | `useUpdates` | 19 |
| `src/tailscale/stream.ts` | `authenticationStream` | 16 |
| `src/tailscale/native.ts` | `nativeTailscale` | 14 |
| `organisms/tailscale/TailscaleConnection.tsx` | `TailscaleConnection` | 13 |
| `src/pages/ProjectsPage.tsx` | `ProjectsPage` | 12 |
| `cockpit/tests/source-boundaries.test.ts` | `allowed` | 11 |
| `internal/acceptance/qemu.go` | `LaunchVM` | 10 |
| `internal/acceptance/runner_vm.go` | `exerciseReusableQCOW2` | 10 |
| `internal/acceptance/scenarios.go` | `exerciseInstalledSystem` | 10 |
| `internal/runners/native.go` | `readDescriptor` | 10 |
| `internal/updates/host.go` | `requireTarget` | 10 |
| `internal/updates/operations.go` | `Download` | 10 |
| `src/projects/ui.ts` | `payloadFor` | 10 |
| `src/projects/ui.ts` | `catalogPayload` | 10 |
| `src/runners/protocol.ts` | `decodeResponse` | 10 |
| `src/runners/ui.ts` | `createPayload` | 10 |
| `src/tailscale/status.ts` | `connectionState` | 10 |

## Suggested Reading Order

| Order | File Path | Risk Level |
| :---: | :--- | :---: |
| 1 | `src/tailscale/types.ts` | **LOW** |
| 2 | `src/runners/types.ts` | **LOW** |
| 3 | `src/projects/types.ts` | **LOW** |
| 4 | `src/updates/types.ts` | **LOW** |
| 5 | `src/atoms/SodaEyebrow.tsx` | **LOW** |
| 6 | `src/tailscale/status.ts` | **HIGH** |
| 7 | `src/tailscale/links.ts` | **LOW** |
| 8 | `src/atoms/ExternalLink.tsx` | **LOW** |
| 9 | `src/runners/ui.ts` | **HIGH** |
| 10 | `src/atoms/CodeValue.tsx` | **LOW** |
| 11 | `src/projects/ui.ts` | **HIGH** |
| 12 | `src/updates/status.ts` | **MEDIUM** |
| 13 | `src/molecules/DiagnosticAlert.tsx` | **LOW** |
| 14 | `src/molecules/PageHeading.tsx` | **LOW** |
| 15 | `src/cockpit/types.ts` | **LOW** |
| 16 | `molecules/tailscale/ExitNodeApprovalGuidance.tsx` | **MEDIUM** |
| 17 | `molecules/tailscale/AuthenticationGuidance.tsx` | **LOW** |
| 18 | `molecules/tailscale/DeviceIdentity.tsx` | **LOW** |
| 19 | `src/molecules/ConfirmationField.tsx` | **LOW** |
| 20 | `molecules/runners/GitHubRegistrationFields.tsx` | **LOW** |
| 21 | `molecules/runners/ForgejoRegistrationFields.tsx` | **LOW** |
| 22 | `molecules/runners/RunnerServiceStatus.tsx` | **LOW** |
| 23 | `molecules/runners/RunnerActions.tsx` | **LOW** |
| 24 | `organisms/projects/dialogTypes.ts` | **LOW** |
| 25 | `molecules/projects/CatalogFields.tsx` | **HIGH** |

*...and 266 more files.*

---

**Generated by Project Gnosis (Code Archaeology Agent)**  
**Repository:** https://github.com/levitateos/soda-os  
**Branch:** main  
**Generated:** 2026-09-05 15:24 UTC  
**Analysis Mode:** Full
