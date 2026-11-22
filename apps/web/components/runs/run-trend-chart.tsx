import type { DailyRunCount } from "@/lib/demo/runs";

const CHART_HEIGHT = 160;

/**
 * Lightweight SVG bar chart of succeeded vs failed runs per day.
 * No charting dependency: values are rendered as accessible bars.
 */
export function RunTrendChart({ data }: { data: DailyRunCount[] }) {
  const max = Math.max(...data.map((d) => d.succeeded + d.failed), 1);

  return (
    <figure className="rounded-lg border border-border bg-card p-4 text-card-foreground">
      <figcaption className="mb-4 flex items-center justify-between">
        <span className="text-sm font-medium">Runs per day</span>
        <span className="flex items-center gap-4 text-xs text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm bg-success" aria-hidden="true" />
            Succeeded
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm bg-error" aria-hidden="true" />
            Failed
          </span>
        </span>
      </figcaption>
      <ul
        className="flex items-end gap-2"
        style={{ height: CHART_HEIGHT }}
        aria-label="Daily run counts"
      >
        {data.map((day) => {
          const successHeight = (day.succeeded / max) * 100;
          const failedHeight = (day.failed / max) * 100;
          return (
            <li
              key={day.date}
              className="flex flex-1 flex-col items-center gap-1"
            >
              <div
                className="flex w-full max-w-10 flex-col justify-end gap-0.5"
                style={{ height: "100%" }}
                role="img"
                aria-label={`${day.date}: ${day.succeeded} succeeded, ${day.failed} failed`}
              >
                <div
                  className="w-full rounded-t-sm bg-error"
                  style={{ height: `${failedHeight}%` }}
                />
                <div
                  className="w-full rounded-b-sm bg-success"
                  style={{ height: `${successHeight}%` }}
                />
              </div>
              <span className="text-[10px] text-muted-foreground">
                {day.date}
              </span>
            </li>
          );
        })}
      </ul>
    </figure>
  );
}
