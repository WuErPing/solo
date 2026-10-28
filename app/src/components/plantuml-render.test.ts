import { beforeEach, describe, expect, it, vi } from "vitest";
import { clearPlantUmlSvgCache, getCachedPlantUmlSvg } from "./plantuml-svg-cache";
import {
  clearPlantUmlInFlight,
  renderPlantUmlSvg,
  type PlantUmlRenderClient,
} from "./plantuml-render";

const SVG = '<svg width="120px" height="80px" viewBox="0 0 120 80"><g/></svg>';

function fakeClient(
  render: PlantUmlRenderClient["plantuml"]["render"],
): PlantUmlRenderClient {
  return { plantuml: { render } };
}

describe("renderPlantUmlSvg", () => {
  beforeEach(() => {
    clearPlantUmlSvgCache();
    clearPlantUmlInFlight();
  });

  it("renders through the daemon, parses, and caches the result", async () => {
    const render = vi.fn().mockResolvedValue({ svg: SVG, error: null });
    const client = fakeClient(render);

    const result = await renderPlantUmlSvg("s1", client, "@startuml\nA -> B\n@enduml", false);

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.entry.widthPx).toBe(120);
      expect(result.entry.heightPx).toBe(80);
    }
    expect(render).toHaveBeenCalledTimes(1);
    expect(getCachedPlantUmlSvg("s1", false, "@startuml\nA -> B\n@enduml")?.widthPx).toBe(120);
  });

  it("serves cache hits without another request", async () => {
    const render = vi.fn().mockResolvedValue({ svg: SVG, error: null });
    const client = fakeClient(render);

    await renderPlantUmlSvg("s1", client, "src", true);
    const second = await renderPlantUmlSvg("s1", client, "src", true);

    expect(second.ok).toBe(true);
    expect(render).toHaveBeenCalledTimes(1);
  });

  it("dedupes concurrent requests for the same diagram", async () => {
    let resolve!: (value: { svg: string; error: string | null }) => void;
    const render = vi.fn().mockImplementation(
      () =>
        new Promise<{ svg: string; error: string | null }>((res) => {
          resolve = res;
        }),
    );
    const client = fakeClient(render);

    const first = renderPlantUmlSvg("s1", client, "src", false);
    const second = renderPlantUmlSvg("s1", client, "src", false);
    expect(render).toHaveBeenCalledTimes(1);

    resolve({ svg: SVG, error: null });
    const [a, b] = await Promise.all([first, second]);
    expect(a.ok).toBe(true);
    expect(b.ok).toBe(true);
  });

  it("returns the daemon error and does not cache it", async () => {
    const client = fakeClient(
      vi.fn().mockResolvedValue({ svg: "", error: "install PlantUML" }),
    );

    const result = await renderPlantUmlSvg("s1", client, "src", false);

    expect(result).toEqual({ ok: false, error: "install PlantUML" });
    expect(getCachedPlantUmlSvg("s1", false, "src")).toBeUndefined();
  });

  it("reports unparseable SVG output", async () => {
    const client = fakeClient(vi.fn().mockResolvedValue({ svg: "nope", error: null }));

    const result = await renderPlantUmlSvg("s1", client, "src", false);

    expect(result.ok).toBe(false);
    expect(getCachedPlantUmlSvg("s1", false, "src")).toBeUndefined();
  });

  it("surfaces request failures", async () => {
    const client = fakeClient(vi.fn().mockRejectedValue(new Error("connection lost")));

    const result = await renderPlantUmlSvg("s1", client, "src", false);

    expect(result).toEqual({ ok: false, error: "connection lost" });
  });
});
