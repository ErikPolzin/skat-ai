import { create } from "zustand";

interface SnackbarState {
  open: boolean;
  message: string;
  severity: "success" | "info" | "warning" | "error";
  showSnackbar: (
    message: string,
    severity?: "success" | "info" | "warning" | "error",
  ) => void;
  hideSnackbar: () => void;
}

export const useSnackbarStore = create<SnackbarState>((set) => ({
  open: false,
  message: "",
  severity: "info",
  showSnackbar: (message, severity = "info") =>
    set({ open: true, message, severity }),
  hideSnackbar: () => set({ open: false }),
}));
