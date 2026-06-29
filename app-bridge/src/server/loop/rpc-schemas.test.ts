import { describe, expect, it } from "vitest";
import {
  LoopListItemSchema,
  LoopRecordSchema,
  LoopRunRequestSchema,
} from "./rpc-schemas.js";
import { AgentSessionConfigSchema } from "../../shared/agent-session-config.js";

const baseLoopRecord = {
  id: "loop-1",
  name: null,
  prompt: "fix tests",
  cwd: "/project",
  provider: "claude",
  model: null,
  workerProvider: null,
  workerModel: null,
  verifierProvider: null,
  verifierModel: null,
  verifyPrompt: null,
  verifyChecks: [],
  archive: false,
  sleepMs: 1000,
  maxIterations: 10,
  maxTimeMs: null,
  status: "running" as const,
  createdAt: "2026-06-29T00:00:00Z",
  updatedAt: "2026-06-29T00:00:00Z",
  startedAt: "2026-06-29T00:00:00Z",
  completedAt: null,
  stopRequestedAt: null,
  iterations: [],
  logs: [],
  nextLogSeq: 1,
  activeIteration: null,
  activeWorkerAgentId: null,
  activeVerifierAgentId: null,
};

describe("LoopRecordSchema", () => {
  it("accepts a legacy record without templates", () => {
    const result = LoopRecordSchema.safeParse(baseLoopRecord);
    expect(result.success).toBe(true);
  });

  it("accepts a record where omitempty deprecated keys are absent (real Go wire shape)", () => {
    // The Go daemon serializes model/workerProvider/workerModel/
    // verifierProvider/verifierModel with `omitempty`, so when unset the keys
    // are missing entirely (not null). The schema must tolerate that or the
    // whole loop/list & loop/run responses get dropped by the client.
    const {
      model: _model,
      workerProvider: _workerProvider,
      workerModel: _workerModel,
      verifierProvider: _verifierProvider,
      verifierModel: _verifierModel,
      ...recordWithoutDeprecatedKeys
    } = baseLoopRecord;
    const result = LoopRecordSchema.safeParse(recordWithoutDeprecatedKeys);
    expect(result.success).toBe(true);
  });

  it("accepts agentTemplate, workerAgentTemplate, and verifierAgentTemplate", () => {
    const result = LoopRecordSchema.safeParse({
      ...baseLoopRecord,
      agentTemplate: {
        provider: "claude",
        cwd: "/project",
        model: "claude-3-opus",
        systemPrompt: "base prompt",
        mcpServers: {
          fs: { type: "stdio", command: "mcp-fs" },
        },
      },
      workerAgentTemplate: {
        provider: "kimi",
        cwd: "/project",
        model: "kimi-k2",
      },
      verifierAgentTemplate: {
        provider: "opencode",
        cwd: "/project",
        model: "deepseek-chat",
      },
    });
    expect(result.success).toBe(true);
    if (!result.success) return;
    expect(result.data.agentTemplate).toBeDefined();
    expect(result.data.workerAgentTemplate).toBeDefined();
    expect(result.data.verifierAgentTemplate).toBeDefined();
    expect(result.data.agentTemplate?.provider).toBe("claude");
    expect(result.data.agentTemplate?.mcpServers?.fs.type).toBe("stdio");
  });

  it("includes AgentSessionConfigSchema fields for templates", () => {
    const agentTemplateShape = AgentSessionConfigSchema.shape;
    const loopShape = LoopRecordSchema.shape;

    expect(loopShape.agentTemplate).toBeDefined();
    expect(loopShape.workerAgentTemplate).toBeDefined();
    expect(loopShape.verifierAgentTemplate).toBeDefined();

    // The template schemas should reference AgentSessionConfigSchema (or an
    // equivalent). A nullable optional wrapper is expected on the record.
    expect(loopShape.agentTemplate?.constructor.name).toBe("ZodNullable");
    expect(loopShape.workerAgentTemplate?.constructor.name).toBe("ZodNullable");
    expect(loopShape.verifierAgentTemplate?.constructor.name).toBe("ZodNullable");

    // AgentSessionConfigSchema itself must include these fields.
    expect(agentTemplateShape.systemPrompt).toBeDefined();
    expect(agentTemplateShape.mcpServers).toBeDefined();
    expect(agentTemplateShape.approvalPolicy).toBeDefined();
  });
});

describe("LoopRunRequestSchema", () => {
  it("accepts a run request with agent templates", () => {
    const result = LoopRunRequestSchema.safeParse({
      type: "loop/run",
      requestId: "req-1",
      prompt: "fix tests",
      cwd: "/project",
      agentTemplate: {
        provider: "claude",
        cwd: "/project",
        model: "claude-3-opus",
      },
      workerAgentTemplate: {
        provider: "kimi",
        cwd: "/project",
      },
    });
    expect(result.success).toBe(true);
  });
});

describe("LoopListItemSchema", () => {
  const baseListItem = {
    id: "loop-1",
    name: null,
    status: "running" as const,
    cwd: "/project",
    provider: "claude",
    model: null,
    createdAt: "2026-06-29T00:00:00Z",
    updatedAt: "2026-06-29T00:00:00Z",
    activeIteration: null,
  };

  it("accepts a list item with model present as null", () => {
    expect(LoopListItemSchema.safeParse(baseListItem).success).toBe(true);
  });

  it("accepts a list item where the omitempty model key is absent (real Go wire shape)", () => {
    const { model: _model, ...itemWithoutModel } = baseListItem;
    expect(LoopListItemSchema.safeParse(itemWithoutModel).success).toBe(true);
  });
});
