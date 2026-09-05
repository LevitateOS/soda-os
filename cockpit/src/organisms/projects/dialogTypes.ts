import type { FormEvent } from "react";
export interface ProjectDialogProps {
  busy: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}
