import type { ReactElement, ReactNode } from "react";
import { Text } from "react-native";
import { describe, expect, it } from "vitest";
import type { ASTNode } from "react-native-markdown-display";
import {
  createSelectableMarkdownTextRules,
  renderSelectableFence,
} from "@/components/markdown-selectable-rules";

type TextElement = ReactElement<{
  selectable?: boolean;
  children?: ReactNode;
  style?: unknown;
}>;

function makeNode(overrides: Partial<ASTNode> = {}): ASTNode {
  return {
    type: "textgroup",
    sourceType: "textgroup",
    key: "7_textgroup",
    content: "",
    markup: "",
    tokenIndex: 0,
    index: 0,
    attributes: {},
    children: [],
    ...overrides,
  };
}

describe("createSelectableMarkdownTextRules", () => {
  it("makes textgroup selectable and keeps children", () => {
    const rules = createSelectableMarkdownTextRules();
    const children: ReactNode[] = ["hello ", "world"];
    const element = rules.textgroup?.(makeNode(), children, [], {}) as TextElement;

    expect(element.type).toBe(Text);
    expect(element.key).toBe("7_textgroup");
    expect(element.props.selectable).toBe(true);
    expect(element.props.children).toEqual(children);
  });

  it("makes code_block selectable, trims the trailing newline and applies styles", () => {
    const rules = createSelectableMarkdownTextRules();
    const node = makeNode({ type: "code_block", key: "9_code_block", content: "const a = 1;\n" });
    const codeBlockStyle = { color: "red" };
    const inheritedStyles = { fontSize: 12 };
    const element = rules.code_block?.(
      node,
      [],
      [],
      { code_block: codeBlockStyle },
      inheritedStyles,
    ) as TextElement;

    expect(element.type).toBe(Text);
    expect(element.key).toBe("9_code_block");
    expect(element.props.selectable).toBe(true);
    expect(element.props.children).toBe("const a = 1;");
    expect(element.props.style).toEqual([inheritedStyles, codeBlockStyle]);
  });

  it("keeps code_block content without a trailing newline untouched", () => {
    const rules = createSelectableMarkdownTextRules();
    const node = makeNode({ type: "code_block", content: "no newline" });
    const element = rules.code_block?.(node, [], [], {}) as TextElement;

    expect(element.props.children).toBe("no newline");
  });
});

describe("renderSelectableFence", () => {
  it("renders a selectable Text with fence styles and trimmed content", () => {
    const node = makeNode({ type: "fence", key: "3_fence", content: "echo hi\n" });
    const fenceStyle = { backgroundColor: "black" };
    const element = renderSelectableFence(node, { fence: fenceStyle }) as TextElement;

    expect(element.type).toBe(Text);
    expect(element.key).toBe("3_fence");
    expect(element.props.selectable).toBe(true);
    expect(element.props.children).toBe("echo hi");
    expect(element.props.style).toEqual([{}, fenceStyle]);
  });
});
