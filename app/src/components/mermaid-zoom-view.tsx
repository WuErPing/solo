// Pinch/pan/double-tap zoom container for a single piece of fixed-size
// content (a mermaid diagram). The content view keeps its natural size and is
// transformed as a whole: scale is anchored to the contain-fit scale so a
// freshly opened diagram fills the viewport without overflow.
//
// The zoom math lives in mermaid-zoom-math.ts (pure, unit-tested); the
// gesture wiring follows the Gesture API + shared-value idiom of
// left-sidebar.tsx.
import React, { useMemo } from "react";
import { StyleSheet, useWindowDimensions, View } from "react-native";
import {
  Gesture,
  GestureDetector,
  GestureHandlerRootView,
} from "react-native-gesture-handler";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import {
  clampOffset,
  clampScale,
  DOUBLE_TAP_ZOOM_FACTOR,
  fitContainScale,
  MAX_ZOOM_FACTOR,
} from "./mermaid-zoom-math";

interface MermaidZoomViewProps {
  /** Natural (unscaled) size of the diagram in CSS px. */
  contentWidth: number;
  contentHeight: number;
  children: React.ReactNode;
}

export function MermaidZoomView({
  contentWidth,
  contentHeight,
  children,
}: MermaidZoomViewProps) {
  const { width: viewportWidth, height: viewportHeight } = useWindowDimensions();
  const minScale = fitContainScale(
    contentWidth,
    contentHeight,
    viewportWidth,
    viewportHeight,
  );
  const maxScale = minScale * MAX_ZOOM_FACTOR;

  const scale = useSharedValue(minScale);
  const savedScale = useSharedValue(minScale);
  const tx = useSharedValue(0);
  const ty = useSharedValue(0);
  const savedTx = useSharedValue(0);
  const savedTy = useSharedValue(0);

  const pinch = useMemo(
    () =>
      Gesture.Pinch()
        .onUpdate((event) => {
          const next = clampScale(savedScale.value * event.scale, minScale, maxScale);
          // Keep the content point under the focal point stationary:
          // t' = f - (f - t) * (s'/s) with f measured from the viewport center.
          const factor = next / savedScale.value;
          const focalX = event.focalX - viewportWidth / 2;
          const focalY = event.focalY - viewportHeight / 2;
          scale.value = next;
          tx.value = clampOffset(
            focalX - (focalX - savedTx.value) * factor,
            contentWidth,
            viewportWidth,
            next,
          );
          ty.value = clampOffset(
            focalY - (focalY - savedTy.value) * factor,
            contentHeight,
            viewportHeight,
            next,
          );
        })
        .onEnd(() => {
          savedScale.value = scale.value;
          savedTx.value = tx.value;
          savedTy.value = ty.value;
        }),
    [
      minScale,
      maxScale,
      viewportWidth,
      viewportHeight,
      contentWidth,
      contentHeight,
      scale,
      savedScale,
      tx,
      ty,
      savedTx,
      savedTy,
    ],
  );

  const pan = useMemo(
    () =>
      Gesture.Pan()
        // Two-finger movement is already handled by the pinch focal math;
        // restricting pan to one pointer avoids double-counting translations.
        .minPointers(1)
        .maxPointers(1)
        .onUpdate((event) => {
          tx.value = clampOffset(
            savedTx.value + event.translationX,
            contentWidth,
            viewportWidth,
            scale.value,
          );
          ty.value = clampOffset(
            savedTy.value + event.translationY,
            contentHeight,
            viewportHeight,
            scale.value,
          );
        })
        .onEnd(() => {
          savedTx.value = tx.value;
          savedTy.value = ty.value;
        }),
    [
      contentWidth,
      contentHeight,
      viewportWidth,
      viewportHeight,
      scale,
      tx,
      ty,
      savedTx,
      savedTy,
    ],
  );

  const doubleTap = useMemo(
    () =>
      Gesture.Tap()
        .numberOfTaps(2)
        .maxDuration(250)
        .onEnd((event) => {
          if (scale.value > minScale * 1.05) {
            scale.value = withTiming(minScale, { duration: 150 });
            tx.value = withTiming(0, { duration: 150 });
            ty.value = withTiming(0, { duration: 150 });
            savedScale.value = minScale;
            savedTx.value = 0;
            savedTy.value = 0;
            return;
          }
          const next = minScale * DOUBLE_TAP_ZOOM_FACTOR;
          const factor = next / scale.value;
          const focalX = event.absoluteX - viewportWidth / 2;
          const focalY = event.absoluteY - viewportHeight / 2;
          const nextTx = clampOffset(
            focalX - (focalX - tx.value) * factor,
            contentWidth,
            viewportWidth,
            next,
          );
          const nextTy = clampOffset(
            focalY - (focalY - ty.value) * factor,
            contentHeight,
            viewportHeight,
            next,
          );
          scale.value = withTiming(next, { duration: 150 });
          tx.value = withTiming(nextTx, { duration: 150 });
          ty.value = withTiming(nextTy, { duration: 150 });
          savedScale.value = next;
          savedTx.value = nextTx;
          savedTy.value = nextTy;
        }),
    [
      minScale,
      viewportWidth,
      viewportHeight,
      contentWidth,
      contentHeight,
      scale,
      savedScale,
      tx,
      ty,
      savedTx,
      savedTy,
    ],
  );

  const composed = useMemo(
    () => Gesture.Simultaneous(pinch, pan, doubleTap),
    [pinch, pan, doubleTap],
  );

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [
      { translateX: tx.value },
      { translateY: ty.value },
      { scale: scale.value },
    ],
  }));

  return (
    <GestureHandlerRootView style={styles.root}>
      <GestureDetector gesture={composed}>
        {/* The gesture surface must fill the viewport: gesture event
            coordinates are relative to the attached view, and the zoom math
            expects window coordinates. */}
        <View style={styles.gestureSurface}>
          <Animated.View
            style={[
              styles.content,
              { width: contentWidth, height: contentHeight },
              animatedStyle,
            ]}
          >
            {children}
          </Animated.View>
        </View>
      </GestureDetector>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
  },
  gestureSurface: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
  },
  content: {
    alignItems: "center",
    justifyContent: "center",
  },
});
