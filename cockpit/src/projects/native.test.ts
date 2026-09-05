import { test, expect, vi } from "vite-plus/test";
import { coordinator } from "./native";
import { pendingProcess } from "../../tests/process";

test("failed removal preserves a complete stdout receipt without parsing stderr", async () => {
  const call = pendingProcess();
  const invoke = coordinator({ spawn: vi.fn(() => call.process) });
  const result = invoke("remove", { id: "site", expected: "a".repeat(64) });
  const receipt = {
    ok: false,
    result: {
      removed: ["alice"],
      uncertain: "bob",
      not_attempted: ["carol"],
      diagnostic: "userdel failed after mutation",
    },
    catalog: "not_attempted",
    problem: "",
  };
  const output = JSON.stringify(receipt);
  call.emit(output.slice(0, 15));
  call.emit(output.slice(15));
  call.reject(new Error("exited with code 1"));
  await expect(result).resolves.toEqual(receipt);
});

test("lost or malformed removal output never becomes success", async () => {
  for (const output of ["", '{"ok":true}', '{"ok":']) {
    const call = pendingProcess();
    const invoke = coordinator({ spawn: vi.fn(() => call.process) });
    const result = invoke("remove-workspace", { id: "site", expected: "a".repeat(64) });
    call.emit(output);
    call.reject(new Error("connection closed"));
    await expect(result).rejects.toBeTruthy();
  }
});
