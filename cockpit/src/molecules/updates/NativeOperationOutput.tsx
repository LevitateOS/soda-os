export function NativeOperationOutput({ output }: { output: string }) {
  return (
    <details open>
      <summary>Native operation output (most recent 16 KiB)</summary>
      <pre className="soda-diagnostic">{output}</pre>
    </details>
  );
}
