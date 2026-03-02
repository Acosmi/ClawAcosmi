import type { GatewayBrowserClient } from "../gateway.ts";

// ---------- Types ----------

export type BackupEntry = {
  index: number;
  path: string;
  size: number;
  modTime: string;
  valid: boolean;
};

export type ResetTarget = {
  path: string;
  size: number;
  exists: boolean;
  action: "delete" | "truncate";
};

export type MaintenanceState = {
  backupsLoading: boolean;
  backups: BackupEntry[];
  backupsError: string | null;
  restoring: boolean;
  restoreResult: string | null;
  resetPreviewLoading: boolean;
  resetTargets: ResetTarget[];
  resetting: boolean;
  resetResult: string | null;
  confirmOpen: boolean;
  confirmAction: "restore" | "reset" | null;
  confirmIndex: number | null;
  confirmText: string;
};

// ---------- Initial State ----------

export function createMaintenanceState(): MaintenanceState {
  return {
    backupsLoading: false,
    backups: [],
    backupsError: null,
    restoring: false,
    restoreResult: null,
    resetPreviewLoading: false,
    resetTargets: [],
    resetting: false,
    resetResult: null,
    confirmOpen: false,
    confirmAction: null,
    confirmIndex: null,
    confirmText: "",
  };
}

// ---------- API Calls ----------

export async function loadBackups(
  client: GatewayBrowserClient,
): Promise<{ backups: BackupEntry[] }> {
  return client.request<{ backups: BackupEntry[] }>("system.backup.list", {});
}

export async function restoreBackup(
  client: GatewayBrowserClient,
  index: number,
): Promise<{ ok: boolean; restoredFrom: number }> {
  return client.request<{ ok: boolean; restoredFrom: number }>(
    "system.backup.restore",
    { index },
  );
}

export async function previewReset(
  client: GatewayBrowserClient,
): Promise<{ level: number; targets: ResetTarget[] }> {
  return client.request<{ level: number; targets: ResetTarget[] }>(
    "system.reset.preview",
    { level: 1 },
  );
}

export async function executeReset(
  client: GatewayBrowserClient,
): Promise<{ ok: boolean; level: number }> {
  return client.request<{ ok: boolean; level: number }>("system.reset", {
    level: 1,
  });
}
