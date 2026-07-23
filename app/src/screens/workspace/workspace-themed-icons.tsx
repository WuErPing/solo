import { ActivityIndicator } from "react-native";
import {
  CopyX,
  ArrowLeftToLine,
  ArrowRightToLine,
  ChevronDown,
  Copy,
  Ellipsis,
  EllipsisVertical,
  PanelRight,
  RotateCw,
  Settings,
  SquarePen,
  SquareTerminal,
  X,
} from "lucide-react-native";
import { withUnistyles } from "react-native-unistyles";
import type { Theme } from "@/styles/theme";
import { SourceControlPanelIcon } from "@/components/icons/source-control-panel-icon";

export const ThemedActivityIndicator = withUnistyles(ActivityIndicator);
export const ThemedEllipsis = withUnistyles(Ellipsis);
export const ThemedEllipsisVertical = withUnistyles(EllipsisVertical);
export const ThemedChevronDown = withUnistyles(ChevronDown);
export const ThemedCopy = withUnistyles(Copy);
export const ThemedRotateCw = withUnistyles(RotateCw);
export const ThemedArrowLeftToLine = withUnistyles(ArrowLeftToLine);
export const ThemedArrowRightToLine = withUnistyles(ArrowRightToLine);
export const ThemedCopyX = withUnistyles(CopyX);
export const ThemedX = withUnistyles(X);
export const ThemedSquarePen = withUnistyles(SquarePen);
export const ThemedSquareTerminal = withUnistyles(SquareTerminal);
export const ThemedSettings = withUnistyles(Settings);
export const ThemedPanelRight = withUnistyles(PanelRight);
export const ThemedSourceControlPanelIcon = withUnistyles(SourceControlPanelIcon);

export const foregroundColorMapping = (theme: Theme) => ({ color: theme.colors.foreground });
export const mutedColorMapping = (theme: Theme) => ({ color: theme.colors.foregroundMuted });

export const sourceControlPanelStrokeWidth15 = { strokeWidth: 1.5 };

export const MENU_NEW_AGENT_ICON = <ThemedSquarePen size={16} uniProps={mutedColorMapping} />;
export const MENU_NEW_TERMINAL_ICON = (
  <ThemedSquareTerminal size={16} uniProps={mutedColorMapping} />
);
export const MENU_COPY_ICON = <ThemedCopy size={16} uniProps={mutedColorMapping} />;
export const MENU_SETTINGS_ICON = <ThemedSettings size={16} uniProps={mutedColorMapping} />;
