// `tailscale up --json` emits successive, indented JSON objects, not JSON lines.
export function authenticationStream(onMessage) {
  let buffer = "";
  let start = 0;
  let depth = 0;
  let quoted = false;
  let escaped = false;
  return {
    push(chunk) {
      const offset = buffer.length;
      buffer += chunk;
      for (let index = offset; index < buffer.length; index++) {
        const character = buffer[index];
        if (quoted) {
          if (escaped) escaped = false;
          else if (character === "\\") escaped = true;
          else if (character === '"') quoted = false;
          continue;
        }
        if (depth === 0) {
          if (/\s/.test(character)) { start = index + 1; continue; }
          if (character !== "{") throw new Error("Invalid Tailscale authentication response.");
          start = index;
        }
        if (character === '"') quoted = true;
        else if (character === "{") depth++;
        else if (character === "}") {
          depth--;
          if (depth === 0) {
            onMessage(JSON.parse(buffer.slice(start, index + 1)));
            start = index + 1;
          }
        }
      }
      if (depth === 0) { buffer = ""; start = 0; }
    },
    finish() {
      if (depth !== 0 || buffer.trim()) throw new Error("Incomplete Tailscale authentication response.");
    },
  };
}
