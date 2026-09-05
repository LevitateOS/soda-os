import { createRoot } from "react-dom/client";
import "../../vendor/cockpit-dark-theme";
import "../../vendor/patternfly/patternfly-6-cockpit.scss";
import "@patternfly/patternfly/patternfly-base.css";
import "../../src/cockpit/soda.css";
import { ProjectsPreview } from "./ProjectsPreview";

createRoot(document.getElementById("app")!).render(<ProjectsPreview />);
