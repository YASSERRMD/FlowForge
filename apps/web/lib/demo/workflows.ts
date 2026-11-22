export type WorkflowStatus = "active" | "draft" | "paused" | "error";

export interface DemoWorkflow {
  id: string;
  name: string;
  description: string;
  status: WorkflowStatus;
  owner: string;
  updatedAt: string;
  runCount: number;
}

/**
 * Static demo data used only until the workflows API exists.
 * Every surface that renders this must label it as demo data.
 */
export const DEMO_WORKFLOWS: DemoWorkflow[] = [
  {
    id: "wf-001",
    name: "Nightly data sync",
    description: "Sync warehouse tables to the reporting store every night.",
    status: "active",
    owner: "Data Platform",
    updatedAt: "2026-11-20T18:30:00Z",
    runCount: 128,
  },
  {
    id: "wf-002",
    name: "Invoice email pipeline",
    description: "Generate and send invoice emails after payment events.",
    status: "active",
    owner: "Billing",
    updatedAt: "2026-11-19T09:12:00Z",
    runCount: 2410,
  },
  {
    id: "wf-003",
    name: "User onboarding",
    description: "Provision accounts and send welcome sequence.",
    status: "draft",
    owner: "Growth",
    updatedAt: "2026-11-18T14:45:00Z",
    runCount: 0,
  },
  {
    id: "wf-004",
    name: "Log archival",
    description: "Archive cold execution logs to object storage.",
    status: "paused",
    owner: "Data Platform",
    updatedAt: "2026-11-12T11:00:00Z",
    runCount: 56,
  },
  {
    id: "wf-005",
    name: "Slack alert router",
    description: "Route incident alerts to the right Slack channel.",
    status: "error",
    owner: "SRE",
    updatedAt: "2026-11-21T07:05:00Z",
    runCount: 812,
  },
  {
    id: "wf-006",
    name: "Weekly KPI digest",
    description: "Compile KPIs and email a Monday-morning digest.",
    status: "active",
    owner: "Analytics",
    updatedAt: "2026-11-17T08:20:00Z",
    runCount: 34,
  },
];
