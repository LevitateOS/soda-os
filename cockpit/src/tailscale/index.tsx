import { createRoot } from "react-dom/client";
import "../../vendor/cockpit-dark-theme";
import "../../vendor/patternfly/patternfly-6-cockpit.scss";
import "@patternfly/patternfly/patternfly-base.css";
import "../cockpit/soda.css";
import { TailscalePage } from "../pages/TailscalePage";
import { nativeTailscale } from "./native";

createRoot(document.getElementById("app")!).render(
  <TailscalePage native={nativeTailscale(window.cockpit)} />,
);
