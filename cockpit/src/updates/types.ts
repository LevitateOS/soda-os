export interface Image {
  version: string;
  imageDigest: string;
  architecture: string;
  image: { image: string; transport: string };
}
export interface Deployment {
  image: Image | null;
  downloadOnly: boolean;
  incompatible: boolean;
}
export interface Host {
  apiVersion: string;
  kind: string;
  status: {
    booted: Deployment;
    staged: Deployment | null;
    rollbackQueued: boolean;
    usrOverlay: unknown;
  };
}
export interface Release {
  version: string;
  revision: string;
  architecture: string;
  reference: string;
  notes_url: string;
}
export interface Selection {
  version: string;
  reference: string;
}
export interface NativeUpdates {
  status(): Promise<Host>;
  check(): Promise<Release>;
  download(selection: Selection, progress: (text: string) => void): Promise<void>;
  apply(selection: Selection, progress: (text: string) => void): Promise<void>;
}
