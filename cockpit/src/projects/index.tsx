import { createRoot } from "react-dom/client";
import "../../vendor/cockpit-dark-theme";
import "../../vendor/patternfly/patternfly-6-cockpit.scss";
import "@patternfly/patternfly/patternfly-base.css";
import "../cockpit/soda.css";
import { ProjectsPage } from "../pages/ProjectsPage";
import { coordinator } from "./native";
import { createProjectsStore } from "./store";

createRoot(document.getElementById("app")!).render(
  <ProjectsPage store={createProjectsStore(coordinator(window.cockpit))} />,
);
