export interface Service {
  load: string;
  active: string;
  sub: string;
  enabled: string;
}
export interface Runner {
  id: string;
  provider: string;
  registration_url: string;
  account: string;
  architecture: string;
  version: string;
  capacity: number;
  service: Service;
}
export interface ListResponse {
  runners: Runner[];
  runner_count: number;
  active_listeners: number;
  total_capacity: number;
}
export interface Registration {
  id: string;
  provider: string;
  registration_url: string;
  registration_id: string;
  labels: string;
  registration_token: string;
}
export interface Requests {
  list: Record<string, never>;
  create: Registration;
  start: { id: string };
  stop: { id: string };
  restart: { id: string };
  remove: { id: string };
}
export type Action = keyof Requests;
export type LifecycleAction = "start" | "stop" | "restart" | "remove";
export type Response<A extends Action> = A extends "list" ? ListResponse : { ok: true };
export type Invoke = <A extends Action>(action: A, payload: Requests[A]) => Promise<Response<A>>;
