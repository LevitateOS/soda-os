## File Explanations

| File | One-line summary | Risk |
| :--- | :--- | :---: |
| [`src/tailscale/types.ts`](#exp-src-tailscale-types-ts) | This file, `cockpit/src/tailscale/types.ts`, is a TypeScript type definition file that exports a set of types used throughout the Tailscale-related components in the Cockpit application. | **LOW** |
| [`src/projects/types.ts`](#exp-src-projects-types-ts) | This file, cockpit/src/projects/types.ts, is responsible for defining type definitions for project-related data structures. | **LOW** |
| [`src/molecules/DiagnosticAlert.tsx`](#exp-src-molecules-diagnosticalert-tsx) | The `DiagnosticAlert` file is responsible for rendering a diagnostic alert component, which displays a message to the user. | **LOW** |
| [`src/projects/ui.ts`](#exp-src-projects-ui-ts) | This file, cockpit/src/projects/ui.ts, is responsible for handling user interface logic for project-related operations. | **HIGH** |
| [`src/runners/types.ts`](#exp-src-runners-types-ts) | This file, `cockpit/src/runners/types.ts`, defines type definitions for the runners module. | **LOW** |
| [`src/updates/types.ts`](#exp-src-updates-types-ts) | This file, cockpit/src/updates/types.ts, defines type definitions for the updates feature in the Cockpit application. | **LOW** |
| [`src/atoms/CodeValue.tsx`](#exp-src-atoms-codevalue-tsx) | The `CodeValue` file is responsible for rendering code snippets within the Cockpit application. | **LOW** |
| [`src/cockpit/types.ts`](#exp-src-cockpit-types-ts) | This file, cockpit/src/cockpit/types.ts, is a TypeScript file that defines type definitions for the Cockpit project. | **LOW** |
| [`src/tailscale/status.ts`](#exp-src-tailscale-status-ts) | This file, `cockpit/src/tailscale/status.ts`, is responsible for managing the Tailscale connection status and related data. | **HIGH** |
| [`src/atoms/ExternalLink.tsx`](#exp-src-atoms-externallink-tsx) | The `ExternalLink` file is responsible for rendering an external link in the application. | **LOW** |
| [`src/runners/ui.ts`](#exp-src-runners-ui-ts) | This file, `cockpit/src/runners/ui.ts`, is responsible for generating UI-related data structures and utility functions for the runners module. | **HIGH** |
| [`src/templates/CockpitPageTemplate.tsx`](#exp-src-templates-cockpitpagetemplate-tsx) | This file, CockpitPageTemplate.tsx, is responsible for rendering the main content of the Cockpit page. | **LOW** |
| [`src/pages/ProjectsPage.tsx`](#exp-src-pages-projectspage-tsx) | The `ProjectsPage` file is responsible for rendering the projects page in the Cockpit application. | **HIGH** |
| [`src/pages/RunnersPage.tsx`](#exp-src-pages-runnerspage-tsx) | The RunnersPage file is responsible for rendering the Runners page, which allows users to create and manage local capacity for provider-owned CI workflows. | **HIGH** |
| [`src/pages/TailscalePage.tsx`](#exp-src-pages-tailscalepage-tsx) | The `TailscalePage` file is responsible for rendering the Tailscale page in the Cockpit application. | **HIGH** |
| [`src/pages/UpdatesPage.tsx`](#exp-src-pages-updatespage-tsx) | The UpdatesPage file is responsible for rendering the updates page in the cockpit application. | **HIGH** |
| [`src/projects/protocol.ts`](#exp-src-projects-protocol-ts) | This file, `protocol.ts`, is responsible for encoding and decoding messages between the cockpit application and a coordinator process. | **HIGH** |
| [`src/runners/protocol.ts`](#exp-src-runners-protocol-ts) | This file, `protocol.ts`, is responsible for encoding and decoding messages exchanged between the cockpit and runner coordinator. | **HIGH** |
| [`src/runners/useRunners.ts`](#exp-src-runners-userunners-ts) | This file, `useRunners.ts`, is responsible for managing the state and behavior of a list of runners in the cockpit application. | **CRITICAL** |
| [`src/tailscale/useTailscale.ts`](#exp-src-tailscale-usetailscale-ts) | This file, `useTailscale.ts`, is responsible for managing the Tailscale connection and providing the necessary data to the application. | **CRITICAL** |

---

### `src/tailscale/types.ts`
**Risk:** LOW · **In-degree:** 15 · **Avg CC:** 0.0

This file, `cockpit/src/tailscale/types.ts`, is a TypeScript type definition file that exports a set of types used throughout the Tailscale-related components in the Cockpit application. Its primary responsibility is to define the shape of data that is used in these components, ensuring that the data conforms to a specific structure and is correctly typed.

### `src/projects/types.ts`
**Risk:** LOW · **In-degree:** 13 · **Avg CC:** 0.0

This file, cockpit/src/projects/types.ts, is responsible for defining type definitions for project-related data structures. It exports a set of TypeScript type interfaces, including Project, ProjectStatus, and CatalogField, which are used throughout the project to ensure type safety and consistency.

### `src/molecules/DiagnosticAlert.tsx`
**Risk:** LOW · **In-degree:** 12 · **Avg CC:** 2.0

The `DiagnosticAlert` file is responsible for rendering a diagnostic alert component, which displays a message to the user. This component is a functional React component that takes in three props: `message`, `variant`, and `role`.

### `src/projects/ui.ts`
**Risk:** HIGH · **In-degree:** 9 · **Avg CC:** 5.3
**Risk Reasons:** 100% of 7 functions have no docstring
> **Depends on:** src/projects/types.ts  
> **Depended on by:** src/pages/ProjectsPage.tsx, src/projects/ui.test.ts, src/projects/useProjects.ts, molecules/projects/ProjectActions.tsx, molecules/projects/WorkspaceSummary.tsx, organisms/projects/CatalogProjectDialog.tsx

This file, cockpit/src/projects/ui.ts, is responsible for handling user interface logic for project-related operations. It exports seven functions: payloadFor, catalogPayload, successMessage, errorMessage, sshCommand, humanDeletionHidden, and projectRemovalHidden. These functions are used to generate payloads for API requests, display success and error messages, construct SSH commands, and determine whether human or project removal is hidden based on the current user's role.

In terms of architecture, this file imports types from ./types and is imported by nine other files, including ProjectsPage.tsx, ui.test.ts, and useProjects.ts. It has a high coupling with the internal file types.ts and a high cyclomatic complexity, with the payloadFor function having a maximum complexity of 10.

Key entry points for a new engineer should be the payloadFor function, which is the primary entry point for generating payloads for API requests, and the catalogPayload function, which is used to construct payloads for catalog-related operations. The successMessage and errorMessage functions are also important, as they are used to display messages to the user.

Before modifying this file, a new engineer should be aware of the high cyclomatic complexity of the payloadFor function and the lack of docstrings for all seven functions. They should also be cautious when modifying the catalogPayload function, as it has a try-catch block that may throw errors if the additional metadata is not a valid JSON object. Additionally, the humanDeletionHidden and projectRemovalHidden functions should be modified with care, as they affect the visibility of human and project removal operations based on the current user's role. A new engineer should read the payloadFor function first to understand the overall logic of the file.

### `src/runners/types.ts`
**Risk:** LOW · **In-degree:** 9 · **Avg CC:** 0.0

This file, `cockpit/src/runners/types.ts`, defines type definitions for the runners module. Its primary responsibility is to provide type annotations for the runners-related data structures and functions, ensuring consistency and correctness throughout the codebase.

### `src/updates/types.ts`
**Risk:** LOW · **In-degree:** 9 · **Avg CC:** 0.0

This file, cockpit/src/updates/types.ts, defines type definitions for the updates feature in the Cockpit application. Its primary responsibility is to provide type annotations for the updates-related data structures, such as the Update and Release types.

### `src/atoms/CodeValue.tsx`
**Risk:** LOW · **In-degree:** 8 · **Avg CC:** 1.0

The `CodeValue` file is responsible for rendering code snippets within the Cockpit application. It exports a single function, `CodeValue`, which takes a `children` prop of type `ReactNode`.

### `src/cockpit/types.ts`
**Risk:** LOW · **In-degree:** 8 · **Avg CC:** 0.0

This file, cockpit/src/cockpit/types.ts, is a TypeScript file that defines type definitions for the Cockpit project. Its primary responsibility is to provide type annotations for various types, interfaces, and enums used throughout the project.

### `src/tailscale/status.ts`
**Risk:** HIGH · **In-degree:** 7 · **Avg CC:** 4.7
**Risk Reasons:** 100% of 7 functions have no docstring
> **Depends on:** src/tailscale/types.ts  
> **Depended on by:** src/tailscale/status.test.ts, src/tailscale/useTailscale.ts, molecules/tailscale/DeviceIdentity.tsx, molecules/tailscale/ExitNodeApprovalGuidance.tsx, organisms/tailscale/ExitNodeForm.tsx, organisms/tailscale/TailscaleConnection.tsx

This file, `cockpit/src/tailscale/status.ts`, is responsible for managing the Tailscale connection status and related data. It exports seven functions that handle various aspects of the connection, including determining the connection state, selecting an exit node, and checking for exit node approval. The file also imports types and data from `./types` and `./types.ts`.

In terms of architecture, this file sits at the center of the Tailscale connection management, importing data from `./types` and being imported by seven other files, including `status.test.ts`, `useTailscale.ts`, and `TailscaleDevices.tsx`. This suggests that it plays a crucial role in the overall Tailscale connection management system.

Key entry points for a new engineer include the `connectionState` function, which determines the current connection state based on the `Status` object, and the `exitSelection` function, which selects an exit node based on the `Snapshot` object. The `exitNodeApproval` function is also an important entry point, as it checks the approval status of the exit node.

However, there are risks and cautions that a new engineer should be aware of before modifying this file. The high cyclomatic complexity of the `connectionState` function (max 10) and the lack of docstrings for all functions (100% of 7 functions have no docstring) make it a challenging file to modify. Additionally, the file imports from only one internal file (`./types.ts`) and is imported by seven other files, which may indicate a high coupling risk. A new engineer should carefully review the code and consider these risks before making any changes. To start, they should read the `connectionState` function to understand the connection state logic.

### `src/atoms/ExternalLink.tsx`
**Risk:** LOW · **In-degree:** 6 · **Avg CC:** 1.0

The `ExternalLink` file is responsible for rendering an external link in the application. It exports a single function, `ExternalLink`, which takes two props: `href` and `children`.

### `src/runners/ui.ts`
**Risk:** HIGH · **In-degree:** 5 · **Avg CC:** 4.6
**Risk Reasons:** 100% of 8 functions have no docstring
> **Depends on:** src/runners/types.ts  
> **Depended on by:** src/runners/ui.test.ts, src/runners/useRunners.ts, molecules/runners/ForgejoRegistrationFields.tsx, molecules/runners/RunnerServiceStatus.tsx, organisms/runners/RunnerCapacity.tsx

This file, `cockpit/src/runners/ui.ts`, is responsible for generating UI-related data structures and utility functions for the runners module. It exports eight functions that handle tasks such as creating payload data for registration, generating status text and class for services, and constructing URLs for providers.

In terms of architecture, this file imports types from `./types.ts` and is imported by five other files, indicating its role as a utility module that provides functionality to other parts of the codebase. The high cyclomatic complexity of the `createPayload` function, with a maximum of 10 conditional statements, is a risk that should be addressed through refactoring.

Key entry points for a new engineer include the `createPayload` function, which is responsible for generating registration data, and the `statusText` and `statusClass` functions, which determine the status of services. The `forgejoBrowserURL` function is also noteworthy, as it constructs URLs for the Forgejo provider.

Before modifying this file, a new engineer should be aware of the high risk of circular dependencies and the lack of docstrings for 100% of the functions, which can make it difficult to understand the purpose and behavior of each function. Additionally, the high cyclomatic complexity of the `createPayload` function makes it a potential source of errors. To get started, a new engineer should read the `createPayload` function first to understand how it generates registration data.

### `src/templates/CockpitPageTemplate.tsx`
**Risk:** LOW · **In-degree:** 5 · **Avg CC:** 2.0

This file, CockpitPageTemplate.tsx, is responsible for rendering the main content of the Cockpit page. It takes in various props such as title, description, actions, feedback, children, dialogs, and busy state, and uses these to construct the page layout.

### `src/pages/ProjectsPage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 12.0
**Risk Reasons:** A function has cyclomatic complexity 12 (threshold: 11) · Average complexity across file is 12.0 (threshold: 10.0) · Imports from 11 unique internal modules (threshold: 8)
> **Depends on:** src/templates/CockpitPageTemplate.tsx, src/molecules/DiagnosticAlert.tsx, organisms/projects/ProjectCatalog.tsx, organisms/projects/PeopleSection.tsx, organisms/projects/CatalogProjectDialog.tsx, organisms/projects/WorkspaceSetupDialog.tsx  
> **Depended on by:** src/pages/ProjectsPage.test.tsx, src/projects/index.tsx

The `ProjectsPage` file is responsible for rendering the projects page in the Cockpit application. It is a React functional component that imports various components and hooks from other files to display a catalog of projects, allow users to add or remove projects, and manage workspace setup. The primary responsibility of this file is to orchestrate the display of projects and handle user interactions.

In terms of architecture, this file sits at the top of the dependency chain, importing components and hooks from other files such as `CockpitPageTemplate`, `DiagnosticAlert`, `ProjectCatalog`, and `useProjects`. It is also imported by two other files, `ProjectsPage.test.tsx` and `index.tsx`, indicating its central role in the application.

Key entry points for a new engineer should be the `ProjectsPage` function and the `useProjects` hook. The `ProjectsPage` function is the main entry point, and understanding its structure and logic is crucial to modifying this file. The `useProjects` hook is also essential, as it provides the data and functionality for the projects page.

There are several risks and cautions that a new engineer should be aware of before modifying this file. The high cyclomatic complexity of the `ProjectsPage` function (12) and the average complexity across the file (12.0) indicate a high risk of errors or bugs. Additionally, the file imports components and hooks from 11 internal files, which can make it difficult to understand the dependencies and relationships between components. Furthermore, the lack of docstrings or comments in some functions and classes may make it challenging to understand their purpose and behavior.

### `src/pages/RunnersPage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 5.0
**Risk Reasons:** Imports from 9 unique internal modules (threshold: 8)
> **Depends on:** src/templates/CockpitPageTemplate.tsx, src/molecules/DiagnosticAlert.tsx, organisms/runners/RunnerCapacity.tsx, organisms/runners/RunnerExecutionNotice.tsx, organisms/runners/ProviderAuthoritySection.tsx, organisms/runners/RegisterRunnerDialog.tsx  
> **Depended on by:** src/pages/RunnersPage.test.tsx, src/runners/index.tsx

The RunnersPage file is responsible for rendering the Runners page, which allows users to create and manage local capacity for provider-owned CI workflows. This file is a React functional component that imports various internal modules to display a complex UI. It uses the useRunners hook to fetch data and manage state, and it renders several child components, including RunnerCapacity, RunnerExecutionNotice, and ProviderAuthoritySection.

In terms of architecture, this file sits at the top of the dependency chain, importing 9 unique internal modules and being imported by 2 other files. Its high coupling and cyclomatic complexity (5.0) indicate that it is a central component that integrates multiple functionalities.

New engineers should start by reading the RunnersPage function, which is the main entry point of this file. They should also examine the useRunners hook, which is responsible for fetching data and managing state. Additionally, they should review the child components, such as RunnerCapacity and RunnerExecutionNotice, to understand how they are used and integrated into the overall UI.

Before modifying this file, engineers should be aware of the following risks: the high coupling and cyclomatic complexity, which can make it difficult to understand and modify; the 9 unique internal module imports, which can lead to tight coupling and make it harder to refactor; and the lack of docstrings, which can make it harder to understand the code. Engineers should also be cautious when modifying the useRunners hook, as it is responsible for fetching data and managing state, and changes to it can have far-reaching consequences.

### `src/pages/TailscalePage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 2.0
**Risk Reasons:** Imports from 9 unique internal modules (threshold: 8)
> **Depends on:** src/templates/CockpitPageTemplate.tsx, src/molecules/DiagnosticAlert.tsx, src/atoms/ExternalLink.tsx, organisms/tailscale/TailscaleConnection.tsx, organisms/tailscale/TailscaleDevices.tsx, organisms/tailscale/ExitNodeForm.tsx  
> **Depended on by:** src/pages/TailscalePage.test.tsx, src/tailscale/index.tsx

The `TailscalePage` file is responsible for rendering the Tailscale page in the Cockpit application. It imports various components from internal modules and uses the `useTailscale` hook to fetch data from the Tailscale API. The file exports a single function, `TailscalePage`, which takes `TailscaleOptions` as a prop and returns a JSX element that represents the Tailscale page.

In terms of architecture, `TailscalePage` sits at the top of the dependency chain, importing components from various internal modules, including `CockpitPageTemplate`, `DiagnosticAlert`, `ExternalLink`, and others. It is also imported by two other files, `TailscalePage.test.tsx` and `index.tsx`.

Key entry points for a new engineer include the `TailscalePage` function itself, as well as the `useTailscale` hook, which is used to fetch data from the Tailscale API. The `CockpitPageTemplate` component is also an important entry point, as it provides the basic structure for the page.

There are several risks and cautions that a new engineer should be aware of when modifying this file. The high coupling between this file and other internal modules makes it difficult to modify without affecting other parts of the application. Additionally, the file has a high cyclomatic complexity, with a maximum complexity of 2 in the `TailscalePage` function. This suggests that the code may be difficult to understand and modify. Finally, the file has no docstrings, which makes it difficult for new engineers to understand the purpose and behavior of the code.

### `src/pages/UpdatesPage.tsx`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 4.0
**Risk Reasons:** Imports from 10 unique internal modules (threshold: 8)
> **Depends on:** src/templates/CockpitPageTemplate.tsx, molecules/updates/UpdateFeedback.tsx, molecules/updates/NativeOperationOutput.tsx, organisms/updates/InstalledImageSection.tsx, organisms/updates/AvailableReleaseSection.tsx, organisms/updates/PendingDeploymentSection.tsx  
> **Depended on by:** src/pages/UpdatesPage.test.tsx, src/updates/index.tsx

The UpdatesPage file is responsible for rendering the updates page in the cockpit application. It imports various components and utilities from internal modules and uses them to display the current updates status, available releases, and pending deployments. The file exports a single function, UpdatesPage, which takes a native object as a prop and returns a JSX element representing the updates page.

In terms of architecture, UpdatesPage sits at the top of the dependency chain, importing components and utilities from various internal modules. It is imported by two files, UpdatesPage.test.tsx and index.tsx, indicating that it is a key component in the cockpit application.

New engineers should start by reading the UpdatesPage function, which is the main entry point of the file. They should also familiarize themselves with the useUpdates hook, which is used to retrieve the current updates state. Additionally, they should understand the role of the CockpitPageTemplate component, which is used to render the page layout.

There are several risks and cautions to be aware of when modifying this file. The high coupling between this file and other internal modules makes it difficult to modify without affecting other parts of the application. The file also has a high cyclomatic complexity, with a maximum of 4 in the UpdatesPage function. Furthermore, the file has no docstrings, which can make it difficult for new engineers to understand the purpose and behavior of the code.

### `src/projects/protocol.ts`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 3.6
**Risk Reasons:** 100% of 8 functions have no docstring
> **Depends on:** src/projects/types.ts  
> **Depended on by:** src/projects/native.ts, src/projects/protocol.test.ts

This file, `protocol.ts`, is responsible for encoding and decoding messages between the cockpit application and a coordinator process. It provides functions to assert the correctness of coordinator responses and to construct requests to the coordinator. The primary functions in this file are `decodeResponse`, `encodeRequest`, and `coordinatorCommand`.

In terms of architecture, this file imports types from `types.ts` and is imported by `native.ts` and `protocol.test.ts`. It sits at the boundary between the cockpit application and the coordinator process, converting between cockpit data structures and coordinator messages.

Key entry points for a new engineer should be the `decodeResponse` function, which handles the parsing and validation of coordinator responses, and the `encodeRequest` function, which constructs requests to the coordinator. The `coordinatorCommand` function is also important, as it generates the command to be sent to the coordinator.

There are several risks and cautions to be aware of when modifying this file. The high cyclomatic complexity of the `decodeResponse` function, with a maximum of 8 conditional statements, makes it a potential source of errors. Additionally, 100% of the functions in this file lack docstrings, making it difficult for a new engineer to understand their purpose and behavior. Finally, the file imports from only one internal file, `types.ts`, but is imported by two other files, `native.ts` and `protocol.test.ts`, which may create a tight coupling between these files.

### `src/runners/protocol.ts`
**Risk:** HIGH · **In-degree:** 2 · **Avg CC:** 4.7
**Risk Reasons:** 100% of 6 functions have no docstring
> **Depends on:** src/runners/types.ts  
> **Depended on by:** src/runners/native.ts, src/runners/protocol.test.ts

This file, `protocol.ts`, is responsible for encoding and decoding messages exchanged between the cockpit and runner coordinator. It provides functions to validate and transform data according to the protocol's rules. The primary functions in this file are `decodeResponse`, `encodeRequest`, and `coordinatorCommand`.

In terms of architecture, this file imports types from `types.ts` and is imported by `native.ts` and `protocol.test.ts`. It sits at the boundary between the cockpit and runner coordinator, translating data between the two systems.

Key entry points for a new engineer include the `decodeResponse` function, which handles the response from the runner coordinator, and the `encodeRequest` function, which prepares the request to be sent to the coordinator. The `coordinatorCommand` function is also important, as it generates the command to be sent to the coordinator.

There are several risks and cautions to be aware of when modifying this file. The high cyclomatic complexity of the `decodeResponse` function, with a maximum of 10 conditional statements, makes it a high-risk area for changes. Additionally, 100% of the functions in this file lack docstrings, making it difficult for new engineers to understand the purpose and behavior of each function. Furthermore, the file imports from only one internal file, but is imported by two other files, which may create a tight coupling between these components.

### `src/runners/useRunners.ts`
**Risk:** CRITICAL · **In-degree:** 1 · **Avg CC:** 28.0
**Risk Reasons:** Function '28.0' has cyclomatic complexity 28 (threshold: 21)
> **Depends on:** src/runners/types.ts, src/runners/ui.ts  
> **Depended on by:** src/pages/RunnersPage.tsx

This file, `useRunners.ts`, is responsible for managing the state and behavior of a list of runners in the cockpit application. It provides a set of functions and state variables that can be used to interact with the runners, including loading, creating, and removing them. The file uses the `Invoke` function to make API calls to the backend, and it also handles errors and notices to display to the user.

In terms of architecture, this file sits at the top of the dependency chain, importing types and UI components from other files. It is imported by the `RunnersPage.tsx` file, which uses the functions and state variables provided by `useRunners` to render the runners list.

Key entry points for a new engineer should be the `useRunners` function itself, as well as the `load`, `refresh`, and `create` functions, which are the core functions that manage the state and behavior of the runners. The `useRunners` function returns a set of state variables and functions that can be used to interact with the runners, including `data`, `busy`, `loading`, `notice`, and `dialog`.

Before modifying this file, a new engineer should be aware of the critical risk of high cyclomatic complexity, with a score of 28, which can make the code difficult to understand and maintain. Additionally, the file has a high coupling factor, importing from two internal files, which can make it harder to modify and test. Finally, the file has no docstrings, which can make it harder for other engineers to understand the purpose and behavior of the code. A new engineer should read the `useRunners` function first to understand the overall architecture and behavior of the file.

### `src/tailscale/useTailscale.ts`
**Risk:** CRITICAL · **In-degree:** 1 · **Avg CC:** 14.3
**Risk Reasons:** Function '36.0' has cyclomatic complexity 36 (threshold: 21)
> **Depends on:** src/cockpit/types.ts, src/tailscale/types.ts, src/tailscale/status.ts  
> **Depended on by:** src/pages/TailscalePage.tsx

This file, `useTailscale.ts`, is responsible for managing the Tailscale connection and providing the necessary data to the application. It uses the `native` object to interact with the Tailscale API and updates the application state accordingly. The file exports a single function, `useTailscale`, which takes an options object as an argument and returns an object with various properties, including the current snapshot, connection state, and authentication URL.

In terms of architecture, this file sits at the top of the dependency chain, importing types and functions from other files, such as `types.ts` and `status.ts`. It is also imported by a single file, `TailscalePage.tsx`, which likely uses the data provided by this file to render the Tailscale page.

Key entry points for a new engineer should be the `useTailscale` function and the `load` function, which is responsible for reading the current Tailscale snapshot and updating the application state. The `load` function is also where the `native` object is used to interact with the Tailscale API.

Before modifying this file, a new engineer should be aware of the following risks: the `useTailscale` function has a cyclomatic complexity of 36, which is above the recommended threshold of 21. This may make it difficult to understand and modify the function without introducing errors. Additionally, the file imports functions from three internal files, which may make it harder to understand the dependencies and interactions between these files.

