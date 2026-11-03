import type { Asset } from "./types";

export function assetStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "pending-launch": "Needs LAUNCH",
    "waiting-4-answers": "Owner input needed",
    "answers-submitted": "Submitted for review",
    "waiting-4-tier-approval": "Awaiting review",
    launched: "Launch complete",
    "pending-tier-validation": "Awaiting validation",
    "pending-shield": "Shield assessment",
    shielded: "Shield complete",
    "pending-security-assessment": "Security assessment",
    "pending-security-implementation": "Implementation",
    implemented: "Implemented",
  };
  return labels[state] ?? state.replaceAll("-", " ");
}

export function launchStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "waiting-4-answers": "Owner input needed",
    "answers-submitted": "Submitted for review",
    "waiting-4-tier-approval": "Awaiting review",
    "tier-validated": "Validated",
    cancelled: "Replaced",
    timedout: "Timed out",
  };
  return labels[state] ?? state.replaceAll("-", " ");
}

export function shieldStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "waiting-4-answers": "Answers in progress",
    "answers-submitted": "Answers submitted",
    "waiting-4-evidences": "Evidence in progress",
    "evidences-submitted": "Finalizing",
    completed: "Completed",
    timedout: "Timed out",
    cancelled: "Replaced",
  };
  return labels[state] ?? state.replaceAll("-", " ");
}

export function locationLabel(asset: Asset): string {
  return [
    asset.campus,
    asset.building,
    asset.floor ? `Floor ${asset.floor}` : "",
    asset.space,
  ]
    .filter(Boolean)
    .join(" · ");
}

export function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat(undefined, {
        month: "short",
        day: "numeric",
        year: "numeric",
      }).format(date);
}
