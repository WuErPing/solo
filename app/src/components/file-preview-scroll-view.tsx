import { useCallback, useState, type ReactNode, type RefObject } from "react";
import {
  ScrollView as RNScrollView,
  View,
  type LayoutChangeEvent,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
  type StyleProp,
  type ViewStyle,
} from "react-native";
import { StyleSheet } from "react-native-unistyles";
import {
  getFilePreviewPagerState,
  type FilePreviewPagerMetrics,
} from "@/components/file-preview-pager";
import { FilePreviewPagerOverlay } from "@/components/file-preview-pager-overlay";

interface PreviewScrollbar {
  onScroll: (event: NativeSyntheticEvent<NativeScrollEvent>) => void;
  onLayout: (event: LayoutChangeEvent) => void;
  onContentSizeChange: (width: number, height: number) => void;
  overlay: ReactNode;
}

interface FilePreviewScrollViewProps {
  scrollRef: RefObject<RNScrollView | null>;
  scrollbar: PreviewScrollbar;
  pagerEnabled: boolean;
  showsVerticalScrollIndicator: boolean;
  contentContainerStyle?: StyleProp<ViewStyle>;
  children: ReactNode;
}

const EMPTY_PAGER_METRICS: FilePreviewPagerMetrics = {
  offset: 0,
  viewportHeight: 0,
  contentHeight: 0,
};

export function FilePreviewScrollView({
  scrollRef,
  scrollbar,
  pagerEnabled,
  showsVerticalScrollIndicator,
  contentContainerStyle,
  children,
}: FilePreviewScrollViewProps) {
  const [pagerMetrics, setPagerMetrics] = useState<FilePreviewPagerMetrics>(EMPTY_PAGER_METRICS);

  const handleScroll = useCallback(
    (event: NativeSyntheticEvent<NativeScrollEvent>) => {
      scrollbar.onScroll(event);
      if (!pagerEnabled) {
        return;
      }
      const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
      setPagerMetrics((prev) =>
        prev.offset === contentOffset.y &&
        prev.viewportHeight === layoutMeasurement.height &&
        prev.contentHeight === contentSize.height
          ? prev
          : {
              offset: contentOffset.y,
              viewportHeight: layoutMeasurement.height,
              contentHeight: contentSize.height,
            },
      );
    },
    [pagerEnabled, scrollbar],
  );

  const handleLayout = useCallback(
    (event: LayoutChangeEvent) => {
      scrollbar.onLayout(event);
      if (!pagerEnabled) {
        return;
      }
      const { height } = event.nativeEvent.layout;
      setPagerMetrics((prev) =>
        prev.viewportHeight === height ? prev : { ...prev, viewportHeight: height },
      );
    },
    [pagerEnabled, scrollbar],
  );

  const handleContentSizeChange = useCallback(
    (width: number, height: number) => {
      scrollbar.onContentSizeChange(width, height);
      if (!pagerEnabled) {
        return;
      }
      setPagerMetrics((prev) =>
        prev.contentHeight === height ? prev : { ...prev, contentHeight: height },
      );
    },
    [pagerEnabled, scrollbar],
  );

  const pagerState = getFilePreviewPagerState(pagerMetrics);

  const handlePageUp = useCallback(() => {
    scrollRef.current?.scrollTo({ y: pagerState.pageUpOffset, animated: true });
  }, [pagerState.pageUpOffset, scrollRef]);

  const handlePageDown = useCallback(() => {
    scrollRef.current?.scrollTo({ y: pagerState.pageDownOffset, animated: true });
  }, [pagerState.pageDownOffset, scrollRef]);

  return (
    <View style={styles.container}>
      <RNScrollView
        ref={scrollRef}
        style={styles.content}
        contentContainerStyle={contentContainerStyle}
        onLayout={handleLayout}
        onScroll={handleScroll}
        onContentSizeChange={handleContentSizeChange}
        scrollEventThrottle={16}
        showsVerticalScrollIndicator={showsVerticalScrollIndicator}
      >
        {children}
      </RNScrollView>
      {scrollbar.overlay}
      {pagerEnabled ? (
        <FilePreviewPagerOverlay
          canPageUp={pagerState.canPageUp}
          canPageDown={pagerState.canPageDown}
          onPageUp={handlePageUp}
          onPageDown={handlePageDown}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create(() => ({
  container: {
    flex: 1,
    minHeight: 0,
  },
  content: {
    flex: 1,
    minHeight: 0,
  },
}));
