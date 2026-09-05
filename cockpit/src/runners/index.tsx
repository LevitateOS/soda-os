import { createRoot } from "react-dom/client";
import "../../vendor/cockpit-dark-theme";
import "../../vendor/patternfly/patternfly-6-cockpit.scss";
import "@patternfly/patternfly/patternfly-base.css";
import "../cockpit/soda.css";
import { RunnersPage } from "../pages/RunnersPage";
import { coordinator } from "./native";

createRoot(document.getElementById("app")!).render(
  <RunnersPage invoke={coordinator(window.cockpit)} />,
);
