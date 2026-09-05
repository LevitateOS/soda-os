export interface Device {
  ID?: string;
  DNSName?: string;
  HostName?: string;
  TailscaleIPs?: string[];
  Expired?: boolean;
  Online?: boolean;
  ExitNodeOption?: boolean;
  InNetworkMap?: boolean;
}
export interface Status {
  BackendState?: string;
  HaveNodeKey?: boolean;
  AuthURL?: string;
  Self?: Device;
  TailscaleIPs?: string[];
  Health?: string[];
  Peer?: Record<string, Device>;
  ExitNodeStatus?: Device;
}
export interface Preferences {
  ExitNodeIP?: string;
  ExitNodeID?: string;
  ExitNodeAllowLANAccess?: boolean;
  AdvertiseRoutes?: string[];
}
export interface AuthenticationMessage extends Status {
  Error?: string;
}
export interface Snapshot {
  status: Status;
  prefs: Preferences;
}
