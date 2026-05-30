import {
  View,
  Text,
  TextInput,
  ActivityIndicator,
  NativeSyntheticEvent,
  TextInputContentSizeChangeEventData,
  TextInputKeyPressEventData,
  TextInputSelectionChangeEventData,
} from "react-native";
import {
  useState,
  useRef,
  useCallback,
  useEffect,
  useLayoutEffect,
  useImperativeHandle,
  useMemo,
  forwardRef,
} from "react";
import { StyleSheet, withUnistyles } from "react-native-unistyles";
import { ICON_SIZE, type Theme } from "@/styles/theme";
import { ArrowUp, CornerDownLeft, Plus } from "lucide-react-native";
import Animated from "react-native-reanimated";
import type { DaemonClient } from "@server/client/daemon-client";
import {
  collectImageFilesFromClipboardData,
  filesToImageAttachments,
} from "@/utils/image-attachments-from-files";
import type { AttachmentMetadata , ComposerAttachment } from "@/attachments/types";
import { focusWithRetries } from "@/utils/web-focus";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Shortcut } from "@/components/ui/shortcut";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWebElementScrollbar } from "@/components/use-web-scrollbar";
import { useShortcutKeys } from "@/hooks/use-shortcut-keys";
import { formatShortcut } from "@/utils/format-shortcut";
import { getShortcutOs } from "@/utils/shortcut-platform";
import type { MessageInputKeyboardActionKind } from "@/keyboard/actions";
import { isImeComposingKeyboardEvent } from "@/utils/keyboard-ime";
import { markScrollInvestigationEvent, markScrollInvestigationRender } from "@/utils/scroll-jank";
import { isWeb } from "@/constants/platform";
import { useComposerHeightMirror } from "./composer-height-mirror";

export type ImageAttachment = AttachmentMetadata;

export interface MessagePayload {
  text: string;
  attachments: ComposerAttachment[];
  cwd: string;
  /** When true, bypasses queue and sends immediately even if agent is running */
  forceSend?: boolean;
}

export interface AttachmentMenuItem {
  id: string;
  label: string;
  onSelect: () => void;
  disabled?: boolean;
  icon?: React.ReactElement | null;
}

export interface MessageInputProps {
  value: string;
  onChangeText: (text: string) => void;
  onSubmit: (payload: MessagePayload) => void;
  /** When true, the submit button is enabled even without text or images (e.g. external attachment selected). */
  hasExternalContent?: boolean;
  /** When true, the submit button stays visible and can submit even with no content. */
  allowEmptySubmit?: boolean;
  /** Optional accessibility label for the primary submit button. */
  submitButtonAccessibilityLabel?: string;
  submitIcon?: "arrow" | "return";
  isSubmitDisabled?: boolean;
  isSubmitLoading?: boolean;
  attachments: ComposerAttachment[];
  cwd: string;
  attachmentMenuItems: AttachmentMenuItem[];
  onAttachButtonRef?: (node: View | null) => void;
  onAddImages?: (images: ImageAttachment[]) => void;
  client: DaemonClient | null;
  placeholder?: string;
  autoFocus?: boolean;
  autoFocusKey?: string;
  disabled?: boolean;
  /** True when this composer's pane is focused. Used to gate global hotkeys when hidden. */
  isPaneFocused?: boolean;
  /** Content to render on the left side of the button row (e.g., AgentStatusBar) */
  leftContent?: React.ReactNode;
  /** Content to render on the right side after send button (e.g., cancel button) */
  rightContent?: React.ReactNode;
  /** When true and there's sendable content, calls onQueue instead of onSubmit */
  isAgentRunning?: boolean;
  /** Controls what the default send action (Enter, send button) does
   *  when the agent is running. "interrupt" sends immediately, "queue" queues. */
  defaultSendBehavior?: "interrupt" | "queue";
  /** Callback for queue button when agent is running */
  onQueue?: (payload: MessagePayload) => void;
  /** Optional handler used when submit button is in loading state. */
  onSubmitLoadingPress?: () => void;
  /** Intercept key press events before default handling. Return true to prevent default. */
  onKeyPress?: (event: { key: string; preventDefault: () => void }) => boolean;
  /** Reports cursor selection updates from the underlying input. */
  onSelectionChange?: (selection: { start: number; end: number }) => void;
  onFocusChange?: (focused: boolean) => void;
  onHeightChange?: (height: number) => void;
  /** Extra styles merged onto the input wrapper (e.g. elevated background). */
  inputWrapperStyle?: import("react-native").ViewStyle;
}

export interface MessageInputRef {
  focus: () => void;
  blur: () => void;
  runKeyboardAction: (action: MessageInputKeyboardActionKind) => boolean;
  /**
   * Web-only: return the underlying DOM element for focus assertions/retries.
   * May return null if not mounted or on native.
   */
  getNativeElement?: () => HTMLElement | null;
}

const MIN_INPUT_HEIGHT_MOBILE = 30;
const MIN_INPUT_HEIGHT_DESKTOP = 46;
const MAX_INPUT_HEIGHT = 160;
const MIN_INPUT_HEIGHT = isWeb ? MIN_INPUT_HEIGHT_DESKTOP : MIN_INPUT_HEIGHT_MOBILE;

type WebTextInputKeyPressEvent = NativeSyntheticEvent<
  TextInputKeyPressEventData & {
    metaKey?: boolean;
    ctrlKey?: boolean;
    shiftKey?: boolean;
    // Web-only: present on DOM KeyboardEvent during IME composition (CJK input).
    isComposing?: boolean;
    keyCode?: number;
  }
>;

interface TextAreaHandle {
  scrollHeight?: number;
  clientHeight?: number;
  offsetHeight?: number;
  scrollTop?: number;
  selectionStart?: number | null;
  selectionEnd?: number | null;
  style?: {
    height?: string;
    overflowY?: string;
  } & Record<string, unknown>;
}

function logWebStickyBottom(_event: string, _details: Record<string, unknown>): void {
  // Intentionally disabled: this path is too noisy during debugging.
}

function getDebugNow(): number | null {
  if (typeof performance !== "undefined" && typeof performance.now === "function") {
    return Number(performance.now().toFixed(3));
  }
  return null;
}

function getElementDescriptor(element: HTMLElement | null): string | null {
  if (!element) return null;
  const tag = element.tagName?.toLowerCase() ?? "unknown";
  const id = element.id ? `#${element.id}` : "";
  const testId = element.getAttribute?.("data-testid");
  const label = element.getAttribute?.("aria-label");
  let suffix: string;
  if (testId) suffix = `[data-testid="${testId}"]`;
  else if (label) suffix = `[aria-label="${label}"]`;
  else suffix = "";
  return `${tag}${id}${suffix}`;
}

function AttachButtonIcon({
  hovered,
  onAttachButtonRef,
  buttonIconSize,
}: {
  hovered: boolean;
  onAttachButtonRef: ((node: View | null) => void) | undefined;
  buttonIconSize: number;
}) {
  const colorMapping = hovered ? iconForegroundMapping : iconForegroundMutedMapping;
  return (
    <View ref={onAttachButtonRef} collapsable={false} style={styles.attachButtonAnchor}>
      <ThemedPlus size={buttonIconSize} uniProps={colorMapping} />
    </View>
  );
}

function AttachmentMenuList({ items }: { items: AttachmentMenuItem[] }) {
  return (
    <>
      {items.map((item) => (
        <DropdownMenuItem
          key={item.id}
          testID={`message-input-attachment-menu-item-${item.id}`}
          disabled={item.disabled}
          onSelect={item.onSelect}
          leading={item.icon ?? null}
        >
          {item.label}
        </DropdownMenuItem>
      ))}
    </>
  );
}

function AttachmentDropdown({
  isConnected,
  disabled,
  attachButtonStyle,
  renderAttachButtonIcon,
  attachmentMenuItems,
}: {
  isConnected: boolean;
  disabled: boolean;
  attachButtonStyle: React.ComponentProps<typeof DropdownMenuTrigger>["style"];
  renderAttachButtonIcon: (input: { hovered?: boolean }) => React.ReactElement;
  attachmentMenuItems: AttachmentMenuItem[];
}) {
  return (
    <DropdownMenu>
      <Tooltip delayDuration={0} enabledOnDesktop enabledOnMobile={false}>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger
            disabled={!isConnected || disabled}
            accessibilityLabel="Add attachment"
            accessibilityRole="button"
            testID="message-input-attach-button"
            style={attachButtonStyle}
          >
            {renderAttachButtonIcon}
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="top" align="center" offset={8}>
          <Text style={styles.tooltipText}>Add attachment</Text>
        </TooltipContent>
      </Tooltip>
      <DropdownMenuContent
        side="top"
        align="start"
        offset={8}
        minWidth={220}
        testID="message-input-attachment-menu"
      >
        <AttachmentMenuList items={attachmentMenuItems} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

type ShortcutChord = NonNullable<React.ComponentProps<typeof Shortcut>["chord"]>;

function SendTooltipBody({
  label,
  sendKeys,
}: {
  label: string;
  sendKeys: ShortcutChord | null | undefined;
}) {
  return (
    <View style={styles.tooltipRow}>
      <Text style={styles.tooltipText}>{label}</Text>
      {sendKeys ? <Shortcut chord={sendKeys} style={styles.tooltipShortcut} /> : null}
    </View>
  );
}

function SendButtonContent({
  isSubmitLoading,
  submitIcon,
  buttonIconSize,
}: {
  isSubmitLoading: boolean;
  submitIcon: "arrow" | "return";
  buttonIconSize: number;
}) {
  if (isSubmitLoading) {
    return <ActivityIndicator size="small" color="white" />;
  }
  if (submitIcon === "return") {
    return <CornerDownLeft size={buttonIconSize} color="white" />;
  }
  return <ArrowUp size={buttonIconSize} color="white" />;
}

function resolveSubmitAccessibilityLabel(input: {
  submitButtonAccessibilityLabel: string | undefined;
  canPressLoadingButton: boolean;
  defaultActionQueues: boolean;
  isAgentRunning: boolean;
}): string {
  if (input.submitButtonAccessibilityLabel) return input.submitButtonAccessibilityLabel;
  if (input.canPressLoadingButton) return "Interrupt agent";
  if (input.defaultActionQueues) return "Queue message";
  if (input.isAgentRunning) return "Send and interrupt";
  return "Send message";
}

function resolveSendTooltipLabel(input: {
  submitButtonAccessibilityLabel: string | undefined;
  defaultActionQueues: boolean;
}): string {
  if (input.submitButtonAccessibilityLabel) return input.submitButtonAccessibilityLabel;
  return input.defaultActionQueues ? "Queue" : "Send";
}

interface DesktopKeyPressContext {
  investigationComponentId: string;
  onKeyPressCallback: ((event: { key: string; preventDefault: () => void }) => boolean) | undefined;
  isAgentRunning: boolean;
  onQueue: ((payload: MessagePayload) => void) | undefined;
  isSubmitDisabled: boolean;
  isSubmitLoading: boolean;
  disabled: boolean;
  handleAlternateSendAction: () => void;
  handleDefaultSendAction: () => void;
}

function handleDesktopKeyPressImpl(
  event: WebTextInputKeyPressEvent,
  ctx: DesktopKeyPressContext,
): void {
  markScrollInvestigationEvent(ctx.investigationComponentId, "keyPress");

  if (isImeComposingKeyboardEvent(event.nativeEvent)) return;

  if (ctx.onKeyPressCallback) {
    const handled = ctx.onKeyPressCallback({
      key: event.nativeEvent.key,
      preventDefault: () => event.preventDefault(),
    });
    if (handled) return;
  }

  const { shiftKey, metaKey, ctrlKey } = event.nativeEvent;

  if (event.nativeEvent.key !== "Enter") return;
  if (shiftKey) return;

  if ((metaKey || ctrlKey) && ctx.isAgentRunning && ctx.onQueue) {
    if (ctx.isSubmitDisabled || ctx.isSubmitLoading || ctx.disabled) return;
    event.preventDefault();
    ctx.handleAlternateSendAction();
    return;
  }

  if (ctx.isSubmitDisabled || ctx.isSubmitLoading || ctx.disabled) return;
  event.preventDefault();
  ctx.handleDefaultSendAction();
}

interface KeyboardActionHandlers {
  textInputRef: React.MutableRefObject<
    TextInput | (TextInput & { getNativeRef?: () => unknown }) | null
  >;
}

function runKeyboardActionImpl(
  action: MessageInputKeyboardActionKind,
  h: KeyboardActionHandlers,
): boolean {
  if (action === "focus") {
    h.textInputRef.current?.focus();
    return true;
  }
  if (action === "send") {
    return false;
  }
  return false;
}

function getTextInputNativeElement(
  current: TextInput | (TextInput & { getNativeRef?: () => unknown }) | null,
): HTMLElement | null {
  if (!current) return null;
  const handle = current as TextInput & { getNativeRef?: () => unknown };
  const native = typeof handle.getNativeRef === "function" ? handle.getNativeRef() : current;
  return native instanceof HTMLElement ? native : null;
}

interface PasteImagesEffectArgs {
  getWebTextArea: () => TextAreaHandle | null;
  isConnected: boolean;
  disabled: boolean;
  onAddImages: ((images: ImageAttachment[]) => void) | undefined;
}

function usePasteImagesEffect(args: PasteImagesEffectArgs): void {
  const {
    getWebTextArea,
    isConnected,
    disabled,
    onAddImages,
  } = args;

  useEffect(() => {
    if (!isWeb || !onAddImages) return;

    const textarea = getWebTextArea() as
      | (TextAreaHandle & {
          addEventListener?: (type: string, listener: (e: ClipboardEvent) => void) => void;
          removeEventListener?: (type: string, listener: (e: ClipboardEvent) => void) => void;
        })
      | null;
    if (
      !textarea ||
      typeof textarea.addEventListener !== "function" ||
      typeof textarea.removeEventListener !== "function"
    ) {
      return;
    }

    let disposed = false;
    const handlePaste = (event: ClipboardEvent) => {
      if (!isConnected || disabled) return;

      const imageFiles = collectImageFilesFromClipboardData(event.clipboardData);
      if (imageFiles.length === 0) return;

      event.preventDefault();

      void filesToImageAttachments(imageFiles)
        .then((pastedAttachments) => {
          if (disposed || pastedAttachments.length === 0) return;
          onAddImages(pastedAttachments);
          return;
        })
        .catch((error) => {
          console.error("[MessageInput] Failed to process pasted images:", error);
        });
    };

    textarea.addEventListener("paste", handlePaste);
    return () => {
      disposed = true;
      textarea.removeEventListener?.("paste", handlePaste);
    };
  }, [
    disabled,
    getWebTextArea,
    isConnected,
    onAddImages,
  ]);
}

interface ResizeObserverEffectArgs {
  getWebTextArea: () => TextAreaHandle | null;
  getWebElement: (target: "root" | "wrapper") => HTMLElement | null;
  valueRef: React.MutableRefObject<string>;
}

function useComposerResizeObserverEffect(args: ResizeObserverEffectArgs): void {
  const { getWebTextArea, getWebElement, valueRef } = args;

  useEffect(() => {
    if (!isWeb || typeof ResizeObserver === "undefined") return;

    const textarea = getWebTextArea();
    const root = getWebElement("root");
    const wrapper = getWebElement("wrapper");
    const observed = [
      { name: "composer_root", element: root },
      { name: "composer_wrapper", element: wrapper },
      { name: "composer_textarea", element: textarea as unknown as HTMLElement | null },
    ].filter(
      (entry): entry is { name: string; element: HTMLElement } =>
        entry.element instanceof HTMLElement,
    );

    if (observed.length === 0) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const target = entry.target as HTMLElement;
        const match = observed.find((item) => item.element === target);
        if (!match) continue;
        const textareaNode = getWebTextArea();
        logWebStickyBottom("composer_element_resized", {
          target: match.name,
          width: target.clientWidth,
          height: target.clientHeight,
          offsetHeight: target.offsetHeight,
          scrollHeight: target.scrollHeight,
          textareaClientHeight: textareaNode?.clientHeight ?? null,
          textareaOffsetHeight: textareaNode?.offsetHeight ?? null,
          textareaScrollHeight: textareaNode?.scrollHeight ?? null,
          textareaScrollTop:
            (textareaNode as unknown as HTMLTextAreaElement | null)?.scrollTop ?? null,
          valueLength: valueRef.current.length,
        });
      }
    });

    for (const entry of observed) {
      observer.observe(entry.element);
    }

    return () => {
      observer.disconnect();
    };
  }, [getWebElement, getWebTextArea, valueRef]);
}

interface ScrollLogEffectArgs {
  getWebTextArea: () => TextAreaHandle | null;
  valueRef: React.MutableRefObject<string>;
}

function useComposerScrollLogEffect(args: ScrollLogEffectArgs): void {
  const { getWebTextArea, valueRef } = args;

  useEffect(() => {
    if (!isWeb) return;
    const textarea = getWebTextArea() as (HTMLTextAreaElement & TextAreaHandle) | null;
    if (!textarea || typeof textarea.addEventListener !== "function") return;

    const handleScroll = () => {
      const textareaElement = textarea as unknown as HTMLElement;
      const chatScroller =
        typeof document !== "undefined"
          ? (document.querySelector('[data-testid="agent-chat-scroll"]') as HTMLElement | null)
          : null;
      logWebStickyBottom("composer_textarea_scrolled", {
        now: getDebugNow(),
        scrollTop: textarea.scrollTop,
        clientHeight: textarea.clientHeight ?? null,
        scrollHeight: textarea.scrollHeight ?? null,
        selectionStart: textarea.selectionStart ?? null,
        selectionEnd: textarea.selectionEnd ?? null,
        textareaDescriptor: getElementDescriptor(textareaElement),
        chatScrollerDescriptor: getElementDescriptor(chatScroller),
        chatScrollerContainsTextarea: Boolean(
          chatScroller && textareaElement && chatScroller.contains(textareaElement),
        ),
        textareaScrollableAncestors: getScrollableAncestorChain(textareaElement),
        valueLength: valueRef.current.length,
      });
    };

    textarea.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      textarea.removeEventListener("scroll", handleScroll);
    };
  }, [getWebTextArea, valueRef]);
}

function useAutoFocusOnWebEffect(
  textInputRef: React.MutableRefObject<
    TextInput | (TextInput & { getNativeRef?: () => unknown }) | null
  >,
  autoFocus: boolean,
  autoFocusKey: string | undefined,
): void {
  useEffect(() => {
    if (!isWeb || !autoFocus) return;
    return focusWithRetries({
      focus: () => textInputRef.current?.focus(),
      isFocused: () => {
        const element = getTextInputNativeElement(textInputRef.current);
        const active = typeof document !== "undefined" ? document.activeElement : null;
        return Boolean(element) && active === element;
      },
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoFocus, autoFocusKey]);
}

function FocusHint({
  visible,
  focusInputKeys,
}: {
  visible: boolean;
  focusInputKeys: ShortcutChord | null | undefined;
}) {
  if (!visible || !focusInputKeys) return null;
  return (
    <Text style={styles.focusHintText} pointerEvents="none">
      {formatShortcut(focusInputKeys[0], getShortcutOs())} to focus
    </Text>
  );
}

function SendButtonTooltip({
  shouldShow,
  canPressLoadingButton,
  onSubmitLoadingPress,
  onDefaultSendAction,
  isSendButtonDisabled,
  submitAccessibilityLabel,
  sendButtonCombinedStyle,
  isSubmitLoading,
  submitIcon,
  buttonIconSize,
  submitButtonAccessibilityLabel,
  defaultActionQueues,
  sendKeys,
}: {
  shouldShow: boolean;
  canPressLoadingButton: boolean;
  onSubmitLoadingPress: (() => void) | undefined;
  onDefaultSendAction: () => void;
  isSendButtonDisabled: boolean;
  submitAccessibilityLabel: string;
  sendButtonCombinedStyle: React.ComponentProps<typeof TooltipTrigger>["style"];
  isSubmitLoading: boolean;
  submitIcon: "arrow" | "return";
  buttonIconSize: number;
  submitButtonAccessibilityLabel: string | undefined;
  defaultActionQueues: boolean;
  sendKeys: ShortcutChord | null | undefined;
}) {
  if (!shouldShow) return null;
  return (
    <Tooltip delayDuration={0} enabledOnDesktop enabledOnMobile={false}>
      <TooltipTrigger
        onPress={canPressLoadingButton ? onSubmitLoadingPress : onDefaultSendAction}
        disabled={isSendButtonDisabled}
        accessibilityLabel={submitAccessibilityLabel}
        accessibilityRole="button"
        style={sendButtonCombinedStyle}
      >
        <SendButtonContent
          isSubmitLoading={isSubmitLoading}
          submitIcon={submitIcon}
          buttonIconSize={buttonIconSize}
        />
      </TooltipTrigger>
      <TooltipContent side="top" align="center" offset={8}>
        <SendTooltipBody
          label={resolveSendTooltipLabel({ submitButtonAccessibilityLabel, defaultActionQueues })}
          sendKeys={sendKeys}
        />
      </TooltipContent>
    </Tooltip>
  );
}

interface SendMessageContext {
  value: string;
  attachments: ComposerAttachment[];
  hasExternalContent: boolean;
  allowEmptySubmit: boolean;
  cwd: string;
  isAgentRunning: boolean;
  onSubmit: (payload: MessagePayload) => void;
  onMinimizeHeight: () => void;
}

function sendMessageImpl(ctx: SendMessageContext): void {
  const trimmed = ctx.value.trim();
  if (
    !trimmed &&
    ctx.attachments.length === 0 &&
    !ctx.hasExternalContent &&
    !ctx.allowEmptySubmit
  ) {
    return;
  }
  ctx.onSubmit({
    text: trimmed,
    attachments: ctx.attachments,
    cwd: ctx.cwd,
    forceSend: ctx.isAgentRunning || undefined,
  });
  ctx.onMinimizeHeight();
}

interface QueueMessageContext {
  value: string;
  attachments: ComposerAttachment[];
  cwd: string;
  onQueue: ((payload: MessagePayload) => void) | undefined;
  onChangeText: (text: string) => void;
  onMinimizeHeight: () => void;
}

function queueMessageImpl(ctx: QueueMessageContext): void {
  if (!ctx.onQueue) return;
  const trimmed = ctx.value.trim();
  if (!trimmed && ctx.attachments.length === 0) return;
  ctx.onQueue({ text: trimmed, attachments: ctx.attachments, cwd: ctx.cwd });
  ctx.onChangeText("");
  ctx.onMinimizeHeight();
}

function computeInvestigationComponentId(): string {
  return "MessageInput";
}

interface SendableContentInput {
  value: string;
  attachments: ComposerAttachment[];
  hasExternalContent: boolean;
  allowEmptySubmit: boolean;
  isSubmitLoading: boolean;
}

interface SendableContentOutput {
  hasAttachments: boolean;
  hasRealContent: boolean;
  hasSendableContent: boolean;
  shouldShowSendButton: boolean;
}

function computeSendableContent(input: SendableContentInput): SendableContentOutput {
  const hasAttachments = input.attachments.length > 0;
  const hasRealContent = input.value.trim().length > 0 || hasAttachments;
  const hasSendableContent = hasRealContent || input.hasExternalContent;
  const shouldShowSendButton =
    hasSendableContent || input.allowEmptySubmit || input.isSubmitLoading;
  return { hasAttachments, hasRealContent, hasSendableContent, shouldShowSendButton };
}

function computeTextInputHeightStyle(inputHeight: number) {
  if (isWeb) {
    return {
      height: inputHeight,
      minHeight: MIN_INPUT_HEIGHT,
      maxHeight: MAX_INPUT_HEIGHT,
    };
  }
  return {
    minHeight: MIN_INPUT_HEIGHT,
    maxHeight: MAX_INPUT_HEIGHT,
  };
}

function isTextAreaLike(v: unknown): v is TextAreaHandle {
  return typeof v === "object" && v !== null && "scrollHeight" in v;
}

function getWebTextAreaImpl(
  current: TextInput | (TextInput & { getNativeRef?: () => unknown }) | null,
): TextAreaHandle | null {
  if (!current) return null;
  const candidate = current as { getNativeRef?: () => unknown };
  if (typeof candidate.getNativeRef === "function") {
    const native = candidate.getNativeRef();
    if (isTextAreaLike(native)) return native;
  }
  if (isTextAreaLike(current)) return current;
  return null;
}

function toHtmlElement(current: unknown): HTMLElement | null {
  if (!current) return null;
  if (current instanceof HTMLElement) return current;
  const maybe = current as { getBoundingClientRect?: () => DOMRect };
  if (maybe.getBoundingClientRect) {
    return current as HTMLElement;
  }
  return null;
}

interface SendButtonStateInput {
  disabled: boolean;
  isSubmitDisabled: boolean;
  isSubmitLoading: boolean;
  onSubmitLoadingPress: (() => void) | undefined;
  defaultSendBehavior: "interrupt" | "queue";
  isAgentRunning: boolean;
}

interface SendButtonStateOutput {
  canPressLoadingButton: boolean;
  isSendButtonDisabled: boolean;
  defaultActionQueues: boolean;
}

function computeSendButtonState(input: SendButtonStateInput): SendButtonStateOutput {
  const canPressLoadingButton =
    input.isSubmitLoading && typeof input.onSubmitLoadingPress === "function";
  const isSendButtonDisabled =
    input.disabled || (!canPressLoadingButton && (input.isSubmitDisabled || input.isSubmitLoading));
  const defaultActionQueues = input.defaultSendBehavior === "queue" && input.isAgentRunning;
  return { canPressLoadingButton, isSendButtonDisabled, defaultActionQueues };
}

interface DefaultSendActionContext {
  defaultSendBehavior: "interrupt" | "queue";
  isAgentRunning: boolean;
  onQueue: ((payload: MessagePayload) => void) | undefined;
  handleSendMessage: () => void;
  handleQueueMessage: () => void;
}

function runDefaultSendAction(ctx: DefaultSendActionContext): void {
  if (ctx.defaultSendBehavior === "queue" && ctx.isAgentRunning && ctx.onQueue) {
    ctx.handleQueueMessage();
    return;
  }
  ctx.handleSendMessage();
}

function runAlternateSendAction(ctx: DefaultSendActionContext): void {
  if (ctx.defaultSendBehavior === "queue") {
    ctx.handleSendMessage();
    return;
  }
  if (ctx.onQueue) {
    ctx.handleQueueMessage();
  }
}

interface ResolvedMessageInputProps {
  value: string;
  onChangeText: (text: string) => void;
  onSubmit: (payload: MessagePayload) => void;
  hasExternalContent: boolean;
  allowEmptySubmit: boolean;
  submitButtonAccessibilityLabel: string | undefined;
  submitIcon: "arrow" | "return";
  isSubmitDisabled: boolean;
  isSubmitLoading: boolean;
  attachments: ComposerAttachment[];
  cwd: string;
  attachmentMenuItems: AttachmentMenuItem[];
  onAttachButtonRef: ((node: View | null) => void) | undefined;
  onAddImages: ((images: ImageAttachment[]) => void) | undefined;
  client: DaemonClient | null;
  placeholder: string;
  autoFocus: boolean;
  autoFocusKey: string | undefined;
  disabled: boolean;
  isPaneFocused: boolean;
  leftContent: React.ReactNode;
  rightContent: React.ReactNode;
  isAgentRunning: boolean;
  defaultSendBehavior: "interrupt" | "queue";
  onQueue: ((payload: MessagePayload) => void) | undefined;
  onSubmitLoadingPress: (() => void) | undefined;
  onKeyPressCallback: ((event: { key: string; preventDefault: () => void }) => boolean) | undefined;
  onSelectionChangeCallback: ((selection: { start: number; end: number }) => void) | undefined;
  onFocusChange: ((focused: boolean) => void) | undefined;
  onHeightChange: ((height: number) => void) | undefined;
  inputWrapperStyle: import("react-native").ViewStyle | undefined;
}

function resolveMessageInputProps(props: MessageInputProps): ResolvedMessageInputProps {
  return {
    value: props.value,
    onChangeText: props.onChangeText,
    onSubmit: props.onSubmit,
    hasExternalContent: props.hasExternalContent ?? false,
    allowEmptySubmit: props.allowEmptySubmit ?? false,
    submitButtonAccessibilityLabel: props.submitButtonAccessibilityLabel,
    submitIcon: props.submitIcon ?? "arrow",
    isSubmitDisabled: props.isSubmitDisabled ?? false,
    isSubmitLoading: props.isSubmitLoading ?? false,
    attachments: props.attachments,
    cwd: props.cwd,
    attachmentMenuItems: props.attachmentMenuItems,
    onAttachButtonRef: props.onAttachButtonRef,
    onAddImages: props.onAddImages,
    client: props.client,
    placeholder: props.placeholder ?? "Message...",
    autoFocus: props.autoFocus ?? false,
    autoFocusKey: props.autoFocusKey,
    disabled: props.disabled ?? false,
    isPaneFocused: props.isPaneFocused ?? true,
    leftContent: props.leftContent,
    rightContent: props.rightContent,
    isAgentRunning: props.isAgentRunning ?? false,
    defaultSendBehavior: props.defaultSendBehavior ?? "interrupt",
    onQueue: props.onQueue,
    onSubmitLoadingPress: props.onSubmitLoadingPress,
    onKeyPressCallback: props.onKeyPress,
    onSelectionChangeCallback: props.onSelectionChange,
    onFocusChange: props.onFocusChange,
    onHeightChange: props.onHeightChange,
    inputWrapperStyle: props.inputWrapperStyle,
  };
}

function getScrollableAncestorChain(element: HTMLElement | null): string[] {
  if (!element || typeof window === "undefined") {
    return [];
  }
  const results: string[] = [];
  let current = element.parentElement;
  while (current) {
    const style = window.getComputedStyle(current);
    const overflowY = style.overflowY;
    const canScroll =
      (overflowY === "auto" || overflowY === "scroll" || overflowY === "overlay") &&
      current.scrollHeight > current.clientHeight;
    if (canScroll) {
      results.push(getElementDescriptor(current) ?? current.tagName.toLowerCase());
    }
    current = current.parentElement;
  }
  return results;
}

export const MessageInput = forwardRef<MessageInputRef, MessageInputProps>(
  function MessageInput(props, ref) {
    const {
      value,
      onChangeText,
      onSubmit,
      hasExternalContent,
      allowEmptySubmit,
      submitButtonAccessibilityLabel,
      submitIcon,
      isSubmitDisabled,
      isSubmitLoading,
      attachments,
      cwd,
      attachmentMenuItems,
      onAttachButtonRef,
      onAddImages,
      client,
      placeholder,
      autoFocus,
      autoFocusKey,
      disabled,
      isPaneFocused,
      leftContent,
      rightContent,
      isAgentRunning,
      defaultSendBehavior,
      onQueue,
      onSubmitLoadingPress,
      onKeyPressCallback,
      onSelectionChangeCallback,
      onFocusChange,
      onHeightChange,
      inputWrapperStyle,
    } = resolveMessageInputProps(props);
    const buttonIconSize = isWeb ? ICON_SIZE.md : ICON_SIZE.lg;
    const investigationComponentId = computeInvestigationComponentId();
    markScrollInvestigationRender(investigationComponentId);
    const sendKeys = useShortcutKeys("message-input-send");
    const focusInputKeys = useShortcutKeys("focus-message-input");
    const [inputHeight, setInputHeight] = useState(MIN_INPUT_HEIGHT);
    const [isInputFocused, setIsInputFocused] = useState(false);
    const rootRef = useRef<View | null>(null);
    const inputWrapperRef = useRef<View | null>(null);
    const textInputRef = useRef<TextInput | (TextInput & { getNativeRef?: () => unknown }) | null>(
      null,
    );
    const isInputFocusedRef = useRef(false);

    useImperativeHandle(ref, () => ({
      focus: () => {
        textInputRef.current?.focus();
      },
      blur: () => {
        textInputRef.current?.blur?.();
      },
      runKeyboardAction: (action) =>
        runKeyboardActionImpl(action, {
          textInputRef,
        }),
      getNativeElement: () => (isWeb ? getTextInputNativeElement(textInputRef.current) : null),
    }));
    const inputHeightRef = useRef(MIN_INPUT_HEIGHT);
    const valueRef = useRef(value);
    useEffect(() => {
      valueRef.current = value;
    }, [value]);

    useEffect(() => {
      return () => {
        onFocusChange?.(false);
      };
    }, [onFocusChange]);

    useAutoFocusOnWebEffect(textInputRef, autoFocus, autoFocusKey);

    const isConnected = client?.isConnected ?? false;

    const minimizeInputHeight = useCallback(() => {
      inputHeightRef.current = MIN_INPUT_HEIGHT;
      setInputHeight(MIN_INPUT_HEIGHT);
      onHeightChange?.(MIN_INPUT_HEIGHT);
    }, [onHeightChange]);

    const handleSendMessage = useCallback(
      () =>
        sendMessageImpl({
          value,
          attachments,
          hasExternalContent,
          allowEmptySubmit,
          cwd,
          isAgentRunning,
          onSubmit,
          onMinimizeHeight: minimizeInputHeight,
        }),
      [
        allowEmptySubmit,
        value,
        attachments,
        cwd,
        onSubmit,
        isAgentRunning,
        hasExternalContent,
        minimizeInputHeight,
      ],
    );

    const handleQueueMessage = useCallback(
      () =>
        queueMessageImpl({
          value,
          attachments,
          cwd,
          onQueue,
          onChangeText,
          onMinimizeHeight: minimizeInputHeight,
        }),
      [value, attachments, cwd, onQueue, onChangeText, minimizeInputHeight],
    );

    const handleDefaultSendAction = useCallback(() => {
      runDefaultSendAction({
        defaultSendBehavior,
        isAgentRunning,
        onQueue,
        handleSendMessage,
        handleQueueMessage,
      });
    }, [defaultSendBehavior, isAgentRunning, onQueue, handleQueueMessage, handleSendMessage]);

    const handleAlternateSendAction = useCallback(() => {
      runAlternateSendAction({
        defaultSendBehavior,
        isAgentRunning,
        onQueue,
        handleSendMessage,
        handleQueueMessage,
      });
    }, [defaultSendBehavior, isAgentRunning, handleSendMessage, handleQueueMessage, onQueue]);

    const getWebTextArea = useCallback(
      (): TextAreaHandle | null => getWebTextAreaImpl(textInputRef.current),
      [],
    );

    const webTextareaRef = useRef<HTMLElement | null>(null);

    useLayoutEffect(() => {
      if (isWeb) {
        webTextareaRef.current = getWebTextArea() as HTMLElement | null;
      }
    }, [getWebTextArea]);

    const inputScrollbar = useWebElementScrollbar(webTextareaRef, {
      enabled: isWeb && inputHeight >= MAX_INPUT_HEIGHT,
    });

    const getWebElement = useCallback((target: "root" | "wrapper"): HTMLElement | null => {
      const current = target === "root" ? rootRef.current : inputWrapperRef.current;
      return toHtmlElement(current);
    }, []);

    usePasteImagesEffect({
      getWebTextArea,
      isConnected,
      disabled,
      onAddImages,
    });

    useComposerResizeObserverEffect({ getWebTextArea, getWebElement, valueRef });

    useComposerScrollLogEffect({ getWebTextArea, valueRef });

    const setBoundedInputHeight = useCallback(
      (nextHeight: number) => {
        const bounded = Math.max(MIN_INPUT_HEIGHT, Math.min(MAX_INPUT_HEIGHT, nextHeight));
        if (Math.abs(inputHeightRef.current - bounded) < 1) return;
        inputHeightRef.current = bounded;
        setInputHeight(bounded);
        onHeightChange?.(bounded);
      },
      [onHeightChange],
    );

    useComposerHeightMirror({
      value,
      textareaRef: webTextareaRef,
      minHeight: MIN_INPUT_HEIGHT,
      maxHeight: MAX_INPUT_HEIGHT,
      onHeight: setBoundedInputHeight,
    });

    const handleContentSizeChange = useCallback(
      (event: NativeSyntheticEvent<TextInputContentSizeChangeEventData>) => {
        if (isWeb) return;
        setBoundedInputHeight(event.nativeEvent.contentSize.height);
      },
      [setBoundedInputHeight],
    );

    const handleSelectionChange = useCallback(
      (event: NativeSyntheticEvent<TextInputSelectionChangeEventData>) => {
        const start = event.nativeEvent.selection?.start ?? 0;
        const end = event.nativeEvent.selection?.end ?? start;
        if (isWeb) {
          const textarea = getWebTextArea();
          logWebStickyBottom("composer_selection_changed", {
            now: getDebugNow(),
            start,
            end,
            textareaScrollTop: textarea?.scrollTop ?? null,
            textareaClientHeight: textarea?.clientHeight ?? null,
            textareaScrollHeight: textarea?.scrollHeight ?? null,
          });
        }
        onSelectionChangeCallback?.({ start, end });
      },
      [getWebTextArea, onSelectionChangeCallback],
    );

    const shouldHandleDesktopSubmit = isWeb;

    function handleDesktopKeyPress(event: WebTextInputKeyPressEvent) {
      if (!shouldHandleDesktopSubmit) return;
      handleDesktopKeyPressImpl(event, {
        investigationComponentId,
        onKeyPressCallback,
        isAgentRunning,
        onQueue,
        isSubmitDisabled,
        isSubmitLoading,
        disabled,
        handleAlternateSendAction,
        handleDefaultSendAction,
      });
    }

    const { shouldShowSendButton } = computeSendableContent({
      value,
      attachments,
      hasExternalContent,
      allowEmptySubmit,
      isSubmitLoading,
    });
    const { canPressLoadingButton, isSendButtonDisabled, defaultActionQueues } =
      computeSendButtonState({
        disabled,
        isSubmitDisabled,
        isSubmitLoading,
        onSubmitLoadingPress,
        defaultSendBehavior,
        isAgentRunning,
      });
    const submitAccessibilityLabel = resolveSubmitAccessibilityLabel({
      submitButtonAccessibilityLabel,
      canPressLoadingButton,
      defaultActionQueues,
      isAgentRunning,
    });

    const handleInputChange = useCallback(
      (nextValue: string) => {
        markScrollInvestigationEvent(investigationComponentId, "inputChange");
        onChangeText(nextValue);
        if (isWeb) {
          logWebStickyBottom("composer_text_changed", {
            valueLength: nextValue.length,
            lineCount: nextValue.split("\n").length,
          });
        }
      },
      [investigationComponentId, onChangeText],
    );

    const handleInputFocus = useCallback(() => {
      isInputFocusedRef.current = true;
      setIsInputFocused(true);
      onFocusChange?.(true);
    }, [onFocusChange]);

    const handleInputBlur = useCallback(() => {
      isInputFocusedRef.current = false;
      setIsInputFocused(false);
      onFocusChange?.(false);
    }, [onFocusChange]);

    const attachButtonStyle = useCallback(
      ({ hovered }: { hovered?: boolean }) => [
        styles.attachButton,
        Boolean(hovered) && styles.iconButtonHovered,
        (!isConnected || disabled) && styles.buttonDisabled,
      ],
      [isConnected, disabled],
    );

    const inputWrapperCombinedStyle = useMemo(
      () => [styles.inputWrapper, inputWrapperStyle],
      [inputWrapperStyle],
    );
    const textInputStyle = useMemo(
      () => [styles.textInput, computeTextInputHeightStyle(inputHeight)],
      [inputHeight],
    );
    const sendButtonCombinedStyle = useMemo(
      () => [styles.sendButton, isSendButtonDisabled && styles.buttonDisabled],
      [isSendButtonDisabled],
    );

    const renderAttachButtonIcon = useCallback(
      ({ hovered }: { hovered?: boolean }) => (
        <AttachButtonIcon
          hovered={Boolean(hovered)}
          onAttachButtonRef={onAttachButtonRef}
          buttonIconSize={buttonIconSize}
        />
      ),
      [onAttachButtonRef, buttonIconSize],
    );

    return (
      <View ref={rootRef} style={styles.container} testID="message-input-root">
        {/* Regular input */}
        <Animated.View ref={inputWrapperRef} style={inputWrapperCombinedStyle}>
          {/* Text input */}
          <View style={styles.textInputScrollWrapper}>
            <ThemedTextInput
              ref={textInputRef}
              value={value}
              onChangeText={handleInputChange}
              placeholder={placeholder}
              uniProps={textInputPlaceholderColorMapping}
              accessibilityLabel="Message agent..."
              onFocus={handleInputFocus}
              onBlur={handleInputBlur}
              style={textInputStyle}
              multiline
              scrollEnabled={isWeb ? inputHeight >= MAX_INPUT_HEIGHT : true}
              onContentSizeChange={handleContentSizeChange}
              editable={!disabled}
              onKeyPress={shouldHandleDesktopSubmit ? handleDesktopKeyPress : undefined}
              onSelectionChange={handleSelectionChange}
              autoFocus={isWeb && autoFocus}
            />
            {inputScrollbar}
            <FocusHint
              visible={isWeb && isPaneFocused && !isInputFocused && !value}
              focusInputKeys={focusInputKeys}
            />
          </View>

          {/* Button row */}
          <View style={styles.buttonRow}>
            {/* Left: attachment button + leftContent slot */}
            <View style={styles.leftButtonGroup}>
              <AttachmentDropdown
                isConnected={isConnected}
                disabled={disabled}
                attachButtonStyle={attachButtonStyle}
                renderAttachButtonIcon={renderAttachButtonIcon}
                attachmentMenuItems={attachmentMenuItems}
              />
              {leftContent}
            </View>

            {/* Right: contextual button (send/cancel) */}
            <View style={styles.rightButtonGroup}>
              {rightContent}
              <SendButtonTooltip
                shouldShow={shouldShowSendButton}
                canPressLoadingButton={canPressLoadingButton}
                onSubmitLoadingPress={onSubmitLoadingPress}
                onDefaultSendAction={handleDefaultSendAction}
                isSendButtonDisabled={isSendButtonDisabled}
                submitAccessibilityLabel={submitAccessibilityLabel}
                sendButtonCombinedStyle={sendButtonCombinedStyle}
                isSubmitLoading={isSubmitLoading}
                submitIcon={submitIcon}
                buttonIconSize={buttonIconSize}
                submitButtonAccessibilityLabel={submitButtonAccessibilityLabel}
                defaultActionQueues={defaultActionQueues}
                sendKeys={sendKeys}
              />
            </View>
          </View>
        </Animated.View>
      </View>
    );
  },
);

const styles = StyleSheet.create((theme: Theme) => ({
  container: {
    position: "relative",
  },
  inputWrapper: {
    flexDirection: "column",
    gap: theme.spacing[3],
    backgroundColor: theme.colors.surface1,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.borderAccent,
    borderRadius: theme.borderRadius["2xl"],
    paddingVertical: {
      xs: theme.spacing[2],
      md: theme.spacing[4],
    },
    paddingHorizontal: {
      xs: theme.spacing[3],
      md: theme.spacing[4],
    },
    ...(isWeb
      ? {
          transitionProperty: "border-color",
          transitionDuration: "200ms",
          transitionTimingFunction: "ease-in-out",
        }
      : {}),
  },
  textInputScrollWrapper: {
    position: "relative",
  },
  focusHintText: {
    position: "absolute",
    top: 0,
    right: 0,
    fontSize: theme.fontSize.xs,
    color: theme.colors.foregroundMuted,
    opacity: 0.5,
  },
  textInput: {
    width: "100%",
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.normal,
    lineHeight: theme.fontSize.base * 1.4,
    ...(isWeb
      ? ({
          outlineStyle: "none",
          outlineWidth: 0,
          outlineColor: "transparent",
        } as object)
      : {}),
  },
  buttonRow: {
    flexDirection: "row",
    alignItems: "flex-end",
    justifyContent: "space-between",
    marginHorizontal: -6,
  },
  leftButtonGroup: {
    minWidth: 0,
    flexShrink: 1,
    flexGrow: 1,
    flexDirection: "row",
    alignItems: "flex-end",
    gap: theme.spacing[0],
  },
  rightButtonGroup: {
    flexShrink: 0,
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[1],
  },
  attachButton: {
    width: 28,
    height: 28,
    borderRadius: theme.borderRadius.full,
    alignItems: "center",
    justifyContent: "center",
  },
  attachButtonAnchor: {
    width: 28,
    height: 28,
    alignItems: "center",
    justifyContent: "center",
  },
  sendButton: {
    width: 28,
    height: 28,
    borderRadius: theme.borderRadius.full,
    backgroundColor: theme.colors.accent,
    alignItems: "center",
    justifyContent: "center",
    marginLeft: theme.spacing[1],
  },
  iconButtonHovered: {
    backgroundColor: theme.colors.surface2,
  },
  tooltipRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  tooltipText: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.popoverForeground,
  },
  tooltipShortcut: {
    backgroundColor: theme.colors.surface3,
    borderColor: theme.colors.borderAccent,
  },
  buttonDisabled: {
    opacity: 0.5,
  },
})) as unknown as Record<string, object>;

const ThemedPlus = withUnistyles(Plus);
const ThemedTextInput = withUnistyles(TextInput);

const iconForegroundMapping = (theme: Theme) => ({ color: theme.colors.foreground });
const iconForegroundMutedMapping = (theme: Theme) => ({ color: theme.colors.foregroundMuted });
const textInputPlaceholderColorMapping = (theme: Theme) => ({
  placeholderTextColor: theme.colors.surface4,
});
