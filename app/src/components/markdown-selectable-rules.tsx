import React from "react";
import { Text, type StyleProp, type TextStyle } from "react-native";
import type { ASTNode, RenderRules } from "react-native-markdown-display";

// The parser appends an extra trailing newline to block code; the library's
// default code_block/fence rules trim it, so the selectable variants do too.
function trimTrailingNewline(content: string): string {
  return content.endsWith("\n") ? content.slice(0, -1) : content;
}

/**
 * Rule overrides that make markdown text selectable on native (long-press →
 * system selection/copy). Only the outermost text containers get `selectable`:
 * nested inline nodes (strong/em/link/code_inline) flatten into the same
 * native text view, so a whole paragraph selects as one unit — adding
 * `selectable` to leaf text nodes would split the view and fragment selection.
 *
 * While these rules are active, link taps inside selectable text are swallowed
 * by the platform's selection handling (Android switches to
 * ArrowKeyMovementMethod; iOS uses the UITextView-backed SelectableParagraph),
 * which is why the file preview gates them behind a select-mode toggle instead
 * of enabling them always-on.
 */
export function createSelectableMarkdownTextRules(): Pick<
  RenderRules,
  "textgroup" | "code_block"
> {
  return {
    textgroup: (
      node: ASTNode,
      children: React.ReactNode[],
      _parentNodes: ASTNode[],
      styles: Record<string, unknown>,
    ) => (
      <Text key={node.key} selectable style={styles.textgroup as StyleProp<TextStyle>}>
        {children}
      </Text>
    ),
    code_block: (
      node: ASTNode,
      _children: React.ReactNode[],
      _parentNodes: ASTNode[],
      styles: Record<string, unknown>,
      inheritedStyles: object = {},
    ) => (
      <Text
        key={node.key}
        selectable
        style={[inheritedStyles, styles.code_block as StyleProp<TextStyle>]}
      >
        {trimTrailingNewline(node.content)}
      </Text>
    ),
  };
}

/** Selectable variant of the library's default fence rule. */
export function renderSelectableFence(
  node: ASTNode,
  styles: Record<string, unknown>,
  inheritedStyles: object = {},
): React.ReactElement {
  return (
    <Text
      key={node.key}
      selectable
      style={[inheritedStyles, styles.fence as StyleProp<TextStyle>]}
    >
      {trimTrailingNewline(node.content)}
    </Text>
  );
}
