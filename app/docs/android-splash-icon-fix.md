# Android 启动画面图标闪烁问题修复

## 问题描述

在 Android 设备上打开应用时，进入主界面前会出现明显的视觉闪烁：从横长方形跳变为正方形。

## 根本原因

启动过程中存在两个阶段使用了不同宽高比的图标：

| 阶段 | 资源 | 尺寸 | 宽高比 |
|------|------|------|--------|
| 原生 Android 启动画面 | `splashscreen_logo.png` (自动生成) | 432×432 | 正方形 ✓ |
| expo-splash-screen | `splash-icon.png` (手动配置) | 1088×608 | 横长方形 ✗ |

两者切换时产生明显的宽高比跳变。

## 解决方案

将 `splash-icon.png` 从横长方形 (1088×608) 替换为正方形 (1024×1024)，与 Android adaptive icon 保持一致。

### 修改前后对比

| 资源文件 | 修改前 | 修改后 |
|---------|--------|--------|
| `assets/images/icon.png` | 1088×608 横长方形 | 不变（用于其他平台） |
| `assets/images/android-icon-foreground.png` | 1024×1024 正方形 | 不变 |
| `assets/images/splash-icon.png` | 1088×608 横长方形 | **1024×1024 正方形** |

### 修改后的启动流程

```
原生启动画面 (正方形) → expo-splash-screen (正方形) → 应用界面
                         ↑ 平滑过渡，无跳变
```

## 执行步骤

1. 复制 `android-icon-foreground.png` 为 `splash-icon.png`：
   ```bash
   cp assets/images/android-icon-foreground.png assets/images/splash-icon.png
   ```

2. 验证尺寸：
   ```bash
   file assets/images/splash-icon.png
   # 输出: PNG image data, 1024 x 1024, 8-bit/color RGBA, non-interlaced
   ```

## 相关配置

### app.config.js (expo-splash-screen 插件)

```javascript
["expo-splash-screen", {
  image: "./assets/images/splash-icon.png",  // 现在是 1024×1024 正方形
  imageWidth: 200,
  resizeMode: "contain",
  backgroundColor: "#ffffff",
  dark: {
    backgroundColor: "#000000",
  },
}]
```

### Android styles.xml (原生启动画面主题)

```xml
<style name="Theme.App.SplashScreen" parent="Theme.SplashScreen">
    <item name="windowSplashScreenBackground">@color/splashscreen_background</item>
    <item name="windowSplashScreenAnimatedIcon">@drawable/splashscreen_logo</item>
    <item name="postSplashScreenTheme">@style/AppTheme</item>
    <item name="android:windowSplashScreenBehavior">icon_preferred</item>
</style>
```

## 验证方法

1. 重新构建 Android 应用：
   ```bash
   npx expo prebuild --clean
   npx expo run:android
   ```

2. 观察启动过程：
   - 原生启动画面应显示正方形 logo
   - expo-splash-screen 应显示相同尺寸的正方形 logo
   - 两者之间无明显跳变

## 注意事项

- `icon.png` (1088×608) 保持不变，它是用于 iOS 和 Web 平台的应用图标
- 此修复仅影响 Android 平台的启动画面显示
- 修改后需要重新执行 `expo prebuild` 以生成新的 Android 资源

## 修改日期

2026-05-19
