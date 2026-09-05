import { afterEach } from "vite-plus/test";
import { cleanup } from "@testing-library/react";
if (typeof document !== "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  afterEach(cleanup);
}
