import React, { useMemo, useState } from "react";
import {
  ActivityIndicator,
  Pressable,
  Text,
  View,
  useWindowDimensions,
} from "react-native";
import { useQuery } from "@tanstack/react-query";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { SvgXml } from "react-native-svg";
import FitImage from "react-native-fit-image";
import { Maximize2 } from "lucide-react-native";
import { Buffer } from "buffer";
import { useSessionStore, type ExplorerFile } from "@/stores/session-store";
import { isAbsolutePath } from "@/utils/path";
import type { AttachmentMetadata } from "@/attachments/types";
import { persistAttachmentFromBase64 } from "@/attachments/service";
import { useAttachmentPreviewUrl } from "@/attachments/use-attachment-preview-url";
import { createPreviewAttachmentId, getFileNameFromPath } from "@/attachments/utils";
import { DiagramLightbox } from "./diagram-lightbox";
import { fitScale } from "./math-svg";
import { parseSvgDocument, type SvgDocument } from "./svg-document";

const SVG_MIME_TYPE = "image/svg+xml";

function toWorkspaceRelative(path: string, workspaceRoot: string): string | null {
  const normalized = path.trim().replace(/\\/g, "/");
  if (!normalized) {
    return null;
  }
  if (!isAbsolutePath(normalized)) {
    return normalized.replace(/^\/+/, "");
  }
  const root = workspaceRoot.trim().replace(/\\/g, "/").replace(/\/+$/, "");
  if (!root) {
    return null;
  }
  if (normalized === root) {
    return "";
  }
  return normalized.startsWith(`${root}/`) ? normalized.slice(root.length + 1) : null;
}

function joinWorkspacePath(directory: string, relative: string): string | null {
  const segments: string[] = [];
  for (const segment of `${directory}/${relative}`.split("/")) {
    if (!segment || segment === ".") {
      continue;
    }
    if (segment === "..") {
      if (segments.length === 0) {
        return null;
      }
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  return segments.length > 0 ? segments.join("/") : null;
}

function cleanImageSrc(src: string): string | null {
  const trimmed = src?.trim();
  if (!trimmed) {
    return null;
  }
  // Any URI scheme (http, https, data, file, windows drive) is not a
  // workspace-relative reference.
  if (/^[a-z][a-z0-9+.-]*:/i.test(trimmed)) {
    return null;
  }
  const withoutSuffix = trimmed.split("#")[0]?.split("?")[0] ?? "";
  let decoded = withoutSuffix;
  try {
    decoded = decodeURIComponent(withoutSuffix);
  } catch {
    // Malformed percent-encoding: fall back to the raw reference.
  }
  const normalized = decoded.trim().replace(/\\/g, "/");
  return normalized || null;
}

/**
 * Resolve a markdown image reference against the document's own directory,
 * returning a workspace-relative path for `exploreFileSystem`, or `null` when
 * the reference is a URL or escapes the workspace root.
 */
export function resolveMarkdownImagePath(
  filePath: string,
  src: string,
  workspaceRoot: string,
): string | null {
  const reference = cleanImageSrc(src);
  if (!reference) {
    return null;
  }
  if (isAbsolutePath(reference)) {
    return toWorkspaceRelative(reference, workspaceRoot) || null;
  }
  const documentPath = toWorkspaceRelative(filePath, workspaceRoot);
  if (documentPath == null) {
    return null;
  }
  const lastSlash = documentPath.lastIndexOf("/");
  const directory = lastSlash >= 0 ? documentPath.slice(0, lastSlash) : "";
  return joinWorkspacePath(directory, reference);
}

/**
 * True when a markdown image reference should be loaded from the workspace
 * instead of the network: the markdown renderer cannot load it itself (it only
 * allows http/https/data URLs, and otherwise prefixes `https://` to the src),
 * and the reference resolves to a file inside the workspace root.
 */
export function isWorkspaceImageReference(
  filePath: string,
  src: string,
  workspaceRoot: string,
  allowedImageHandlers: string[],
): boolean {
  const uri = src.toLowerCase();
  const loadable = allowedImageHandlers.some((value) => uri.startsWith(value.toLowerCase()));
  if (loadable) {
    return false;
  }
  return resolveMarkdownImagePath(filePath, src, workspaceRoot) !== null;
}

function decodeSvgFile(file: ExplorerFile): string | null {
  if (!file.content) {
    return null;
  }
  if (file.encoding === "base64") {
    try {
      return Buffer.from(file.content, "base64").toString("utf8");
    } catch {
      return null;
    }
  }
  return file.content;
}

interface SvgFigureProps {
  svg: SvgDocument;
  alt?: string;
}

function SvgFigure({ svg, alt }: SvgFigureProps) {
  const { theme } = useUnistyles();
  const { width: windowWidth } = useWindowDimensions();
  const [expanded, setExpanded] = useState(false);
  // Matches the markdown scroll content padding (see file-pane
  // previewMarkdownScrollContent) minus the container border on each side.
  const contentWidth = Math.max(0, windowWidth - 2 * theme.spacing[4] - 2);
  const scale = fitScale(svg.widthPx, contentWidth);

  return (
    <View style={styles.figure} testID="markdown-file-image-svg">
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={alt ? `Expand ${alt}` : "Expand image"}
        onPress={() => setExpanded(true)}
        style={styles.figureContent}
      >
        <SvgXml xml={svg.xml} width={svg.widthPx * scale} height={svg.heightPx * scale} />
        <View
          style={[
            styles.expandBadge,
            { backgroundColor: theme.colors.surface2, pointerEvents: "none" },
          ]}
        >
          <Maximize2 size={12} color={theme.colors.foregroundMuted} />
        </View>
      </Pressable>
      {expanded ? (
        <DiagramLightbox
          xml={svg.xml}
          widthPx={svg.widthPx}
          heightPx={svg.heightPx}
          onClose={() => setExpanded(false)}
        />
      ) : null}
    </View>
  );
}

interface MarkdownFileImageProps {
  src: string;
  alt?: string;
  serverId: string;
  workspaceRoot: string;
  filePath: string;
}

/**
 * Renders a markdown image whose `src` is a workspace-relative path (for
 * example `imgs/architecture.svg`). The bytes are fetched from the daemon with
 * the same query key the file pane uses, so a file already open in a preview is
 * served from cache. SVG documents are painted with react-native-svg; raster
 * images go through the attachment store like any other preview.
 */
export function MarkdownFileImage({
  src,
  alt,
  serverId,
  workspaceRoot,
  filePath,
}: MarkdownFileImageProps) {
  const resolvedPath = useMemo(
    () => resolveMarkdownImagePath(filePath, src, workspaceRoot),
    [filePath, src, workspaceRoot],
  );
  const client = useSessionStore((state) => state.sessions[serverId]?.client ?? null);

  const query = useQuery({
    queryKey: ["workspaceFile", serverId, workspaceRoot, resolvedPath],
    enabled: Boolean(client && resolvedPath),
    queryFn: async (): Promise<{
      svg: SvgDocument | null;
      attachment: AttachmentMetadata | null;
      error: string | null;
    }> => {
      if (!client || !resolvedPath) {
        return { svg: null, attachment: null, error: "Image path is outside the workspace." };
      }
      const payload = await client.exploreFileSystem(workspaceRoot, resolvedPath, "file");
      const file = payload.file ?? null;
      if (!file) {
        return { svg: null, attachment: null, error: payload.error ?? "Image not found." };
      }

      const svgSource =
        file.mimeType === SVG_MIME_TYPE || file.kind === "text" ? decodeSvgFile(file) : null;
      if (svgSource) {
        const parsed = parseSvgDocument(svgSource);
        if (parsed) {
          return { svg: parsed, attachment: null, error: null };
        }
      }

      if (file.kind === "image" && file.content) {
        const attachment = await persistAttachmentFromBase64({
          id: createPreviewAttachmentId({
            mimeType: file.mimeType ?? "image/png",
            path: file.path,
            size: file.size,
            modifiedAt: file.modifiedAt,
            contentLength: file.content.length,
          }),
          base64: file.content,
          mimeType: file.mimeType,
          fileName: getFileNameFromPath(file.path),
        });
        return { svg: null, attachment, error: null };
      }

      return { svg: null, attachment: null, error: payload.error ?? "Image preview unavailable." };
    },
    staleTime: 5_000,
  });

  const attachmentUri = useAttachmentPreviewUrl(query.data?.attachment ?? null);

  if (resolvedPath && query.isLoading) {
    return (
      <View style={styles.placeholder} testID="markdown-file-image-loading">
        <ActivityIndicator size="small" />
      </View>
    );
  }

  const svg = query.data?.svg ?? null;
  if (svg) {
    return <SvgFigure svg={svg} alt={alt} />;
  }

  if (attachmentUri) {
    return (
      <View style={styles.figure} testID="markdown-file-image-raster">
        <FitImage
          indicator
          source={{ uri: attachmentUri }}
          accessible={alt ? true : undefined}
          accessibilityLabel={alt ?? undefined}
        />
      </View>
    );
  }

  return (
    <View style={styles.missing} testID="markdown-file-image-missing">
      <Text style={styles.missingText} numberOfLines={2}>
        {alt || resolvedPath || src}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  figure: {
    marginVertical: 8,
    borderRadius: 8,
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "rgba(128,128,128,0.2)",
  },
  figureContent: {
    alignItems: "center",
    paddingVertical: 16,
  },
  expandBadge: {
    position: "absolute",
    top: 6,
    right: 6,
    width: 24,
    height: 24,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
    opacity: 0.85,
  },
  placeholder: {
    height: 120,
    alignItems: "center",
    justifyContent: "center",
  },
  missing: {
    marginVertical: 8,
    padding: theme.spacing[3],
    borderRadius: 8,
    borderWidth: 1,
    borderColor: theme.colors.border,
    backgroundColor: theme.colors.surface0,
  },
  missingText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
}));
