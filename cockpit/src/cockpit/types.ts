/** Only the documented Cockpit browser API surface used by these packages. */
export interface CockpitProcess extends PromiseLike<string> {
  input(data: string): CockpitProcess;
  stream(callback: (chunk: string) => void): CockpitProcess;
  close(reason?: string): void;
}
export interface SpawnOptions {
  err: "message" | "out";
  superuser?: "require";
}
export interface HttpRequest {
  method: string;
  path: string;
  body: string;
  headers?: Record<string, string>;
}
export interface CockpitHTTP {
  request(options: HttpRequest): PromiseLike<string>;
  close(reason?: string): void;
}
export interface Cockpit {
  spawn(command: string[], options: SpawnOptions): CockpitProcess;
  http(
    path: string,
    options: { superuser: "require"; headers: Record<string, string> },
  ): CockpitHTTP;
  hidden: boolean;
  addEventListener(type: "visibilitychange", callback: () => void): void;
  removeEventListener(type: "visibilitychange", callback: () => void): void;
}
declare global {
  interface Window {
    cockpit: Cockpit;
    debugging?: string;
  }
}
