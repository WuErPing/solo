import { describe, expect, it } from "vitest";
import { filterSlashCommands, AGENT_COMMANDS } from "./agent-commands";

describe("filterSlashCommands", () => {
  it("returns empty when input does not start with /", () => {
    expect(filterSlashCommands("claude", "compact")).toEqual([]);
    expect(filterSlashCommands("claude", "")).toEqual([]);
    expect(filterSlashCommands("claude", "help")).toEqual([]);
  });

  it("returns all commands for agent when input is just /", () => {
    const result = filterSlashCommands("claude", "/");
    expect(result.length).toBe(AGENT_COMMANDS.claude.length);
    expect(result[0].command).toBe("/compact");
  });

  it("filters by prefix when query is provided", () => {
    const result = filterSlashCommands("claude", "/co");
    expect(result.map((c) => c.command)).toEqual(["/compact", "/config", "/cost"]);
  });

  it("returns single result for exact match", () => {
    const result = filterSlashCommands("claude", "/help");
    expect(result).toEqual([{ label: "help", command: "/help" }]);
  });

  it("returns empty for unknown agent", () => {
    expect(filterSlashCommands("unknown", "/")).toEqual([]);
    expect(filterSlashCommands("unknown", "/help")).toEqual([]);
  });

  it("returns empty when no commands match prefix", () => {
    expect(filterSlashCommands("claude", "/zzz")).toEqual([]);
  });

  it("matches case-insensitively", () => {
    const result = filterSlashCommands("claude", "/Compact");
    expect(result).toEqual([{ label: "compact", command: "/compact" }]);
  });
});
