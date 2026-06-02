import { describe, it, expect } from "vitest";
import {
  detectTimezone,
  cronToUTC,
  cronFromUTC,
  formatCronWithTimezone,
  describeCron,
} from "../cron-timezone";

describe("detectTimezone", () => {
  it("returns a string", () => {
    const tz = detectTimezone();
    expect(typeof tz).toBe("string");
    expect(tz.length).toBeGreaterThan(0);
  });
});

describe("cronToUTC", () => {
  it("returns expression unchanged for UTC timezone", () => {
    expect(cronToUTC("0 9 * * *", "UTC")).toBe("0 9 * * *");
  });

  it("returns expression unchanged for empty timezone", () => {
    expect(cronToUTC("0 9 * * *", "")).toBe("0 9 * * *");
  });

  it("converts 9am UTC+8 to 1am UTC", () => {
    // Asia/Shanghai is UTC+8, so 9am local = 1am UTC
    const result = cronToUTC("0 9 * * *", "Asia/Shanghai");
    expect(result).toBe("0 1 * * *");
  });

  it("converts 9am UTC-5 (EST) to 2pm UTC", () => {
    // America/New_York is UTC-5 (EST) or UTC-4 (EDT)
    // We use the offset at the current date, so just test the structure
    const result = cronToUTC("0 9 * * *", "America/New_York");
    // The exact result depends on DST, but it should be a valid cron
    const parts = result.split(" ");
    expect(parts.length).toBe(5);
    expect(parts[0]).toBe("0"); // minute unchanged
  });

  it("handles hour wrapping (midnight crossing)", () => {
    // UTC+9, 2am local → 2-9 = -7 → 17 UTC (previous day)
    const result = cronToUTC("0 2 * * *", "Asia/Tokyo");
    expect(result).toBe("0 17 * * *");
  });

  it("preserves minute and other fields", () => {
    const result = cronToUTC("30 9 * * 1", "Asia/Shanghai");
    expect(result).toBe("30 1 * * 1");
  });

  it("handles wildcard hour", () => {
    expect(cronToUTC("* * * * *", "Asia/Shanghai")).toBe("* * * * *");
  });

  it("handles step-wildcard hour", () => {
    expect(cronToUTC("0 */2 * * *", "Asia/Shanghai")).toBe("0 */2 * * *");
  });

  it("handles list hours", () => {
    // 9,12,15 in UTC+8 → 1,4,7 in UTC
    const result = cronToUTC("0 9,12,15 * * *", "Asia/Shanghai");
    expect(result).toBe("0 1,4,7 * * *");
  });

  it("handles range hours", () => {
    // 9-17 in UTC+8 → 1-9 in UTC
    const result = cronToUTC("0 9-17 * * *", "Asia/Shanghai");
    expect(result).toBe("0 1-9 * * *");
  });
});

describe("cronFromUTC", () => {
  it("returns expression unchanged for UTC timezone", () => {
    expect(cronFromUTC("0 1 * * *", "UTC")).toBe("0 1 * * *");
  });

  it("converts 1am UTC to 9am UTC+8", () => {
    const result = cronFromUTC("0 1 * * *", "Asia/Shanghai");
    expect(result).toBe("0 9 * * *");
  });

  it("is the inverse of cronToUTC", () => {
    const original = "0 9 * * *";
    const utc = cronToUTC(original, "Asia/Shanghai");
    const back = cronFromUTC(utc, "Asia/Shanghai");
    expect(back).toBe(original);
  });
});

describe("formatCronWithTimezone", () => {
  it("returns raw expression for UTC", () => {
    expect(formatCronWithTimezone("0 9 * * *", "UTC")).toBe("0 9 * * *");
  });

  it("returns raw expression for undefined timezone", () => {
    expect(formatCronWithTimezone("0 9 * * *")).toBe("0 9 * * *");
  });

  it("appends timezone for non-UTC", () => {
    expect(formatCronWithTimezone("0 9 * * *", "Asia/Shanghai")).toBe(
      "0 9 * * * (Asia/Shanghai)",
    );
  });
});

describe("describeCron", () => {
  it("describes daily cron", () => {
    expect(describeCron("0 9 * * *")).toBe("每天 09:00");
  });

  it("describes daily cron with different time", () => {
    expect(describeCron("30 14 * * *")).toBe("每天 14:30");
  });

  it("describes hourly cron", () => {
    expect(describeCron("0 * * * *")).toBe("每小时 :00");
  });

  it("describes hourly cron with minute offset", () => {
    expect(describeCron("30 * * * *")).toBe("每小时 :30");
  });

  it("describes weekly cron on Monday", () => {
    expect(describeCron("0 9 * * 1")).toBe("每周一 09:00");
  });

  it("describes weekly cron on Friday", () => {
    expect(describeCron("0 18 * * 5")).toBe("每周五 18:00");
  });

  it("describes weekly cron on Sunday", () => {
    expect(describeCron("0 10 * * 0")).toBe("每周日 10:00");
  });

  it("returns raw expression for complex patterns", () => {
    expect(describeCron("*/5 * * * *")).toBe("*/5 * * * *");
  });

  it("returns raw expression for range patterns", () => {
    expect(describeCron("0 9-17 * * *")).toBe("0 9-17 * * *");
  });

  it("returns raw expression for invalid input", () => {
    expect(describeCron("bad")).toBe("bad");
  });
});
