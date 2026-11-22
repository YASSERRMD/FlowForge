export interface RunStats {
  total: number;
  succeeded: number;
  failed: number;
  running: number;
  avgDurationSeconds: number;
}

export interface DailyRunCount {
  date: string;
  succeeded: number;
  failed: number;
}

/**
 * Static demo data used only until the runs API exists.
 * Every surface that renders this must label it as demo data.
 */
export const DEMO_RUN_STATS: RunStats = {
  total: 3440,
  succeeded: 3208,
  failed: 197,
  running: 35,
  avgDurationSeconds: 42,
};

export const DEMO_RUN_TREND: DailyRunCount[] = [
  { date: "Nov 15", succeeded: 380, failed: 22 },
  { date: "Nov 16", succeeded: 402, failed: 18 },
  { date: "Nov 17", succeeded: 396, failed: 31 },
  { date: "Nov 18", succeeded: 441, failed: 25 },
  { date: "Nov 19", succeeded: 467, failed: 29 },
  { date: "Nov 20", succeeded: 512, failed: 34 },
  { date: "Nov 21", succeeded: 610, failed: 38 },
];
