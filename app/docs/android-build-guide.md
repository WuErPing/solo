# Android 构建与安装指南

本文档记录本项目 Android APK 的完整构建与安装流程，包括 EAS 云端构建和本地 Gradle 构建两种方式。

---

## 目录

1. [环境准备](#环境准备)
2. [EAS 云端构建（推荐）](#eas-云端构建推荐)
3. [本地 Gradle 构建](#本地-gradle-构建)
4. [常见问题与修复](#常见问题与修复)
5. [安装到设备](#安装到设备)
6. [图标规范](#图标规范)

---

## 环境准备

### 必需环境

| 组件 | 版本 | 说明 |
|---|---|---|
| Java | 17 | OpenJDK 17 |
| Android SDK | — | 需包含以下组件 |
| — platform-tools | 37.0.0+ | `adb`、`fastboot` 等 |
| — platforms;android-36 | API 36 | 编译目标平台 |
| — build-tools;36.0.0 | 36.0.0 | 主要构建工具 |
| — build-tools;35.0.0 | 35.0.0 | 部分依赖需要 |
| — cmake;3.22.1 | 3.22.1 | React Native C++ 编译 |
| — NDK | 27.0.12077973 (r27) | 原生代码编译 |

### 环境变量

```bash
export ANDROID_HOME="$HOME/Library/Android/sdk"
export ANDROID_SDK_ROOT="$ANDROID_HOME"
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/27.0.12077973"
export PATH="$ANDROID_HOME/platform-tools:$PATH"
```

### 安装 SDK 组件（macOS）

```bash
# 1. 下载 Android Command Line Tools
mkdir -p "$ANDROID_HOME/cmdline-tools"
cd "$ANDROID_HOME/cmdline-tools"
curl -L -o cmdline-tools.zip \
  "https://dl.google.com/android/repository/commandlinetools-mac-11076708_latest.zip"
unzip -q cmdline-tools.zip
mv cmdline-tools latest

# 2. 安装必需组件
./latest/bin/sdkmanager --install \
  "platform-tools" \
  "platforms;android-36" \
  "build-tools;36.0.0" \
  "build-tools;35.0.0" \
  "cmake;3.22.1"

# 3. 安装 NDK（手动下载 DMG）
# NDK r27: https://dl.google.com/android/repository/android-ndk-r27-darwin.dmg
# 挂载后将 NDK 目录复制到 $ANDROID_HOME/ndk/27.0.12077973
```

---

## EAS 云端构建（推荐）

适合不想配置本地 Android 环境的场景。构建在 Expo 服务器上完成，产物通过链接下载。

### 配置

`app/eas.json` 已配置 `development` profile：

```json
{
  "build": {
    "development": {
      "developmentClient": true,
      "distribution": "internal",
      "channel": "development",
      "env": { "APP_VARIANT": "development" },
      "android": { "gradleCommand": ":app:assembleDebug" }
    }
  }
}
```

### 提交构建

```bash
cd app
npx eas build --platform android --profile development
```

### 状态查询

```bash
npx eas build:view <build-id>
```

### 特点

- 无需本地 Android SDK
- 自动管理签名证书（Keystore）
- 构建排队时间取决于服务器负载（10~30 分钟）
- 产物为 Debug APK，内置 `expo-dev-client`

---

## 本地 Gradle 构建

适合需要快速迭代、调试原生代码或离线工作的场景。

### Debug 构建（开发客户端）

```bash
cd app/android
./gradlew :app:assembleDebug --no-daemon
```

产物：`app/android/app/build/outputs/apk/debug/app-debug.apk`

特点：
- 包含 `expo-dev-client`
- 可连接 Metro 开发服务器
- 使用 debug 签名

### Release 构建（生产版本）

```bash
cd app/android
./gradlew :app:assembleRelease --no-daemon
```

产物：`app/android/app/build/outputs/apk/release/app-release.apk`

特点：
- 不包含开发客户端
- 独立运行，无需 Metro
- 当前使用 debug 签名（生产环境应配置独立 Keystore）

### 首次构建时长

- Debug：约 15~30 分钟（首次，需下载依赖）
- Release：约 4~6 分钟（增量构建约 20~30 秒）

---

## 常见问题与修复

### 1. NDK 版本不匹配

**错误**：
```
NDK from ndk.dir had version [27.0.12077973] which disagrees with android.ndkVersion [27.1.12297006]
```

**修复**：在 `app/android/gradle.properties` 中覆盖版本：

```properties
ndkVersion=27.0.12077973
```

### 2. react-native-unistyles 编译错误

**错误**：
```
Unresolved reference 'CxxPart'
'initHybrid' overrides nothing
```

**原因**：`react-native-unistyles` 3.2.4 需要 `react-native-nitro-modules` 0.35.5，但项目锁定在 0.33.8。

**修复**：升级 `react-native-nitro-modules`：

```bash
cd app
npm install react-native-nitro-modules@0.35.5
```

### 3. SDK 组件缺失

构建过程中可能依次提示缺少以下组件，逐一安装即可：

```bash
# build-tools
sdkmanager --install "build-tools;36.0.0"
sdkmanager --install "build-tools;35.0.0"

# platform
sdkmanager --install "platforms;android-36"

# platform-tools
sdkmanager --install "platform-tools"

# cmake
sdkmanager --install "cmake;3.22.1"
```

---

## 安装到设备

### 通过 ADB 安装

```bash
# 安装 Debug APK
adb install app/android/app/build/outputs/apk/debug/app-debug.apk

# 安装 Release APK（覆盖安装）
adb install -r app/android/app/build/outputs/apk/release/app-release.apk

# 完全卸载后重装（清除缓存）
adb uninstall sh.solo
adb install app/android/app/build/outputs/apk/release/app-release.apk
```

### 通过 Gradle 直接安装并运行

```bash
cd app/android
./gradlew :app:installDebug
./gradlew :app:installRelease
```

### 指定设备（多设备时）

```bash
adb -s <device-id> install app-release.apk
```

### 启动应用

```bash
# 启动到主界面
adb shell am start -n sh.solo/.MainActivity

# 启动开发客户端并连接 Metro
adb shell am start -a android.intent.action.VIEW \
  -d "exp+voice-mobile://expo-development-client/?url=http%3A%2F%2F<ip>%3A8082" \
  sh.solo
```

---

## 图标规范

### Android 自适应图标要求

| 项目 | 规范 |
|---|---|
| 画布尺寸 | 1024×1024 px（源文件） |
| 安全区域 | 画布中心直径 66% 的圆形区域 |
| 内容放置 | 核心图形必须位于安全区域内 |
| 比例 | 必须为 **1:1 正方形** |
| 背景色 | 在 `app.config.js` 中配置 `backgroundColor` |

### 本项目配置

```javascript
// app.config.js
android: {
  adaptiveIcon: {
    backgroundColor: "#000000",
    foregroundImage: "./assets/images/android-icon-foreground.png",
  },
}
```

### 图标处理流程

原始素材 `icon.png` 为 **1088×608 横长方形**，需处理为正方形：

1. **提取圆形区域**：从原图裁剪以太极图为中心的正方形区域
2. **去除外围背景**：将太极图外部的白色背景替换为黑色（与背景色一致）
3. **缩放到安全区域**：624×624 px（1024 的 61%），居中放置
4. **生成多密度资源**：
   - 前景图（108/162/216/324/432 dp）
   - 普通图标 + 圆形图标（48/72/96/144/192 dp）

### 注意

- 不要提供非正方形的前景图，Android 自适应图标系统会裁剪异常
- 图标资源需同步到 `android/app/src/main/res/mipmap-*/` 各密度目录
- 修改源文件后需重新构建 APK，完全卸载安装以清除 launcher 缓存

---

## 相关文件

| 文件 | 说明 |
|---|---|
| `app/eas.json` | EAS 构建配置 |
| `app/app.config.js` | Expo 项目配置（含图标、scheme） |
| `app/android/app/build.gradle` | Android 应用构建配置 |
| `app/android/gradle.properties` | Gradle 属性（NDK 版本覆盖等） |
| `app/assets/images/android-icon-foreground.png` | Android 自适应图标前景 |
| `app/assets/images/icon.png` | 通用应用图标源文件 |
