export interface ImageReference {
  image: string;
  transport: string;
}
export interface Image {
  version: string | null;
  imageDigest: string;
  architecture: string;
  image: ImageReference;
}
export interface Deployment {
  image: Image | null;
  cachedUpdate: Image | null;
  downloadOnly: boolean;
  incompatible: boolean;
}
export interface Host {
  apiVersion: string;
  kind: string;
  spec: { image: ImageReference | null };
  status: {
    booted: Deployment;
    staged: Deployment | null;
    rollbackQueued: boolean;
    usrOverlay: unknown;
    readOnly: boolean;
  };
}
export interface NativeUpdates {
  status(): Promise<Host>;
  check(): Promise<Host>;
  update(progress: (text: string) => void): Promise<void>;
}
