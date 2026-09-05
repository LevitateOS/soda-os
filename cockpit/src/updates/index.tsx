import { createRoot } from "react-dom/client";
import "../../vendor/cockpit-dark-theme";
import "../../vendor/patternfly/patternfly-6-cockpit.scss";
import "@patternfly/patternfly/patternfly-base.css";
import "../cockpit/soda.css";
import { UpdatesPage } from "../pages/UpdatesPage";
import { nativeUpdates } from "./native";

createRoot(document.getElementById("app")!).render(
  <UpdatesPage native={nativeUpdates(window.cockpit)} />,
);
