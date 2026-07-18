import http from "node:http";
import type { AddressInfo } from "node:net";

export interface RecordedChatCompletionRequest {
  method: string;
  path: string;
  /** Raw request body as received. */
  rawBody: string;
  /** Parsed JSON body (null when the body is not valid JSON). */
  json: unknown;
  /** Authorization header value, e.g. "Bearer test-key". */
  authorization: string | null;
  /** Requested model id from the body, when present. */
  model: string | null;
}

export type StubLlmHandler = (body: unknown) => string;

function buildChatCompletionBody(content: string): string {
  return JSON.stringify({
    id: "chatcmpl-stub",
    object: "chat.completion",
    created: 0,
    model: "stub-model",
    choices: [
      {
        index: 0,
        message: { role: "assistant", content },
        finish_reason: "stop",
      },
    ],
    usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 },
  });
}

/**
 * Minimal OpenAI-compatible chat-completions endpoint for schedule-assistant
 * e2e tests. Point the daemon's llmProviders config at `baseURL`; the
 * assistant message content is programmable per test and every request is
 * recorded for optional assertions.
 *
 * Lifecycle: `start()` / `close()` per test — never leave servers running.
 */
export class StubLlmServer {
  private server: http.Server | null = null;
  private handler: StubLlmHandler = () => "{}";
  private recorded: RecordedChatCompletionRequest[] = [];
  private port = 0;

  /** Base URL to configure as the daemon LLM provider baseURL. */
  get baseURL(): string {
    return `http://127.0.0.1:${this.port}/v1`;
  }

  /** All chat-completion requests received so far, in order. */
  get requests(): readonly RecordedChatCompletionRequest[] {
    return this.recorded;
  }

  /** Return the same assistant content for every subsequent request. */
  setContent(content: string): void {
    this.handler = () => content;
  }

  /** Compute the assistant content from each request body. */
  setHandler(handler: StubLlmHandler): void {
    this.handler = handler;
  }

  async start(): Promise<void> {
    if (this.server) {
      return;
    }
    const server = http.createServer((req, res) => this.handleRequest(req, res));
    this.server = server;
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    const address = server.address() as AddressInfo | null;
    if (!address) {
      throw new Error("Stub LLM server failed to bind a port");
    }
    this.port = address.port;
  }

  async close(): Promise<void> {
    const server = this.server;
    this.server = null;
    if (!server) {
      return;
    }
    // The daemon's HTTP client reuses keep-alive connections; force-close them
    // so `close()` settles promptly instead of waiting for idle timeouts.
    (server as http.Server & { closeAllConnections?: () => void }).closeAllConnections?.();
    await new Promise<void>((resolve) => {
      server.close(() => resolve());
    });
  }

  private handleRequest(req: http.IncomingMessage, res: http.ServerResponse): void {
    const path = (req.url ?? "").split("?")[0];
    const isChatCompletion =
      req.method === "POST" &&
      (path === "/v1/chat/completions" || path === "/chat/completions");
    if (!isChatCompletion) {
      res.writeHead(404, { "content-type": "application/json" });
      res.end(
        JSON.stringify({ error: { message: `stub-llm: unhandled ${req.method} ${path}` } }),
      );
      return;
    }

    let rawBody = "";
    req.setEncoding("utf8");
    req.on("data", (chunk: string) => {
      rawBody += chunk;
    });
    req.on("end", () => {
      let json: unknown = null;
      try {
        json = JSON.parse(rawBody);
      } catch {
        // Keep json null — the daemon's client always sends valid JSON.
      }
      const model =
        json !== null && typeof (json as { model?: unknown }).model === "string"
          ? ((json as { model: string }).model as string)
          : null;
      this.recorded.push({
        method: req.method ?? "",
        path,
        rawBody,
        json,
        authorization: req.headers.authorization ?? null,
        model,
      });

      let content: string;
      try {
        content = this.handler(json);
      } catch (error) {
        res.writeHead(500, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            error: { message: `stub-llm handler failed: ${String(error)}` },
          }),
        );
        return;
      }

      res.writeHead(200, { "content-type": "application/json" });
      res.end(buildChatCompletionBody(content));
    });
  }
}
