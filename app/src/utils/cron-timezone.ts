/**
 * Utilities for converting cron expressions between local timezone and UTC.
 *
 * Only the hour field is adjusted. Minute, dom, month, dow are unchanged.
 * Cron format: "minute hour dom month dow"
 */

/** Detect the user's IANA timezone (e.g. "Asia/Shanghai"). */
export function detectTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return "UTC";
  }
}

/** Get UTC offset in hours for a timezone at a given date. Positive = east of UTC. */
function getUtcOffsetHours(timezone: string, date: Date): number {
  const tzDate = new Date(
    date.toLocaleString("en-US", { timeZone: timezone }),
  );
  const utcDate = new Date(date.toLocaleString("en-US", { timeZone: "UTC" }));
  return (tzDate.getTime() - utcDate.getTime()) / 3600000;
}

/** Wrap hour to 0-23 range. */
function wrapHour(hour: number): number {
  return ((hour % 24) + 24) % 24;
}

/**
 * Adjust the hour field in a cron expression by a given offset.
 * Handles wildcards, lists, ranges, and steps. Only numeric hours are shifted.
 */
function adjustCronHour(expression: string, offsetHours: number): string {
  const parts = expression.trim().split(/\s+/);
  if (parts.length < 2) return expression;

  parts[1] = adjustHourField(parts[1], offsetHours);
  return parts.join(" ");
}

function adjustHourField(hourField: string, offset: number): string {
  // Wildcard or step-wildcard: */N — timezone shift doesn't apply meaningfully
  if (hourField === "*") return "*";

  // Step with wildcard: */N
  if (/^\*\/\d+$/.test(hourField)) return hourField;

  // List: 1,5,10
  if (hourField.includes(",")) {
    return hourField
      .split(",")
      .map((h) => String(wrapHour(parseInt(h, 10) + offset)))
      .join(",");
  }

  // Range: 9-17
  if (hourField.includes("-") && !hourField.includes("/")) {
    const [start, end] = hourField.split("-").map((h) => parseInt(h, 10));
    return `${wrapHour(start + offset)}-${wrapHour(end + offset)}`;
  }

  // Range with step: 9-17/2
  if (hourField.includes("-") && hourField.includes("/")) {
    const [range, step] = hourField.split("/");
    const [start, end] = range.split("-").map((h) => parseInt(h, 10));
    return `${wrapHour(start + offset)}-${wrapHour(end + offset)}/${step}`;
  }

  // Simple number
  const num = parseInt(hourField, 10);
  if (Number.isNaN(num)) return hourField;
  return String(wrapHour(num + offset));
}

/**
 * Convert a cron expression from a given timezone to UTC.
 * e.g. "0 9 * * *" in Asia/Shanghai (UTC+8) → "0 1 * * *" in UTC
 */
export function cronToUTC(expression: string, timezone: string): string {
  if (!timezone || timezone === "UTC") return expression;
  const offset = getUtcOffsetHours(timezone, new Date());
  if (offset === 0) return expression;
  return adjustCronHour(expression, -offset);
}

/**
 * Convert a cron expression from UTC to a given timezone.
 * e.g. "0 1 * * *" in UTC → "0 9 * * *" in Asia/Shanghai (UTC+8)
 */
export function cronFromUTC(expression: string, timezone: string): string {
  if (!timezone || timezone === "UTC") return expression;
  const offset = getUtcOffsetHours(timezone, new Date());
  if (offset === 0) return expression;
  return adjustCronHour(expression, offset);
}

/**
 * Format a cron expression for display with timezone info.
 * e.g. "0 9 * * *" + "Asia/Shanghai" → "0 9 * * * (Asia/Shanghai)"
 */
export function formatCronWithTimezone(expression: string, timezone?: string): string {
  if (!timezone || timezone === "UTC") return expression;
  return `${expression} (${timezone})`;
}

const DOW_NAMES = ["日", "一", "二", "三", "四", "五", "六"];

/**
 * Describe a (local) cron expression in human-readable Chinese text.
 * Assumes the expression is already in the user's local timezone.
 *
 * - "0 9 * * *"   → "每天 09:00"
 * - "30 * * * *"  → "每小时 :30"
 * - "0 9 * * 1"   → "每周一 09:00"
 * - other patterns → raw expression
 */
export function describeCron(expression: string): string {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 5) return expression;
  const [m, h, dom, mon, dow] = parts;

  const minuteNum = parseInt(m, 10);
  const hourNum = parseInt(h, 10);
  const pad = (n: number) => String(n).padStart(2, "0");

  // Hourly: M * * * *  (M must be a plain number)
  if (h === "*" && dom === "*" && mon === "*" && dow === "*" && /^\d+$/.test(m)) {
    return `每小时 :${pad(minuteNum)}`;
  }

  // Daily: M H * * *  (H and M must be plain numbers, not range/list/step)
  if (dom === "*" && mon === "*" && dow === "*" && /^\d+$/.test(h) && /^\d+$/.test(m)) {
    return `每天 ${pad(hourNum)}:${pad(minuteNum)}`;
  }

  // Weekly: M H * * DOW  (H and M must be plain numbers)
  if (dom === "*" && mon === "*" && dow !== "*" && /^\d+$/.test(h) && /^\d+$/.test(m)) {
    const dowNum = parseInt(dow, 10);
    if (!Number.isNaN(dowNum) && dowNum >= 0 && dowNum <= 6) {
      return `每周${DOW_NAMES[dowNum]} ${pad(hourNum)}:${pad(minuteNum)}`;
    }
  }

  return expression;
}
