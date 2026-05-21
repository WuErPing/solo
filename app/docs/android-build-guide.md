# Android 真机构建与发布指南

> 适用场景：修改 JS/组件代码（如 About Solo 页面加 logo）后，将更新发布到 Android 真机。

---

## 1. 项目构建体系概述

本项目基于 **Expo + EAS (Expo Application Services)** 构建，支持三种方式将代码变更同步到真机：

| 方式 | 适用场景 | 是否需要重新安装 App | 耗时 |
|------|---------|-------------------|------|
| Metro Reload (`r`) | 本地开发调试 | 否 | 秒级 |
| EAS Update (OTA) | JS/资源热更新，已安装用户自动接收 | 否 | 2-5 分钟 |
| 本地 Gradle Build | 不走远程服务，本地构建并安装 | 是 | 3-10 分钟 |
| EAS Build | 原生代码变更、生成新 APK、新用户安装 | 是 | 10-30 分钟 |

关键配置文件：
- `app/eas.json` — EAS 构建配置（development / production / production-apk）
- `app/app.config.js` — Expo 应用配置（含 EAS Update URL、runtimeVersion）
- `app/package.json` — 构建脚本定义

---

## 2. 方式一：开发调试（Metro Reload）

适用于：本地开发阶段，修改 React 组件/JS 代码后即时验证。

### 前提条件
- 真机已安装 **Development Client**（通过 `eas build --profile development` 构建并安装）
- 电脑与真机处于同一局域网
- Metro 开发服务器已启动（`npm run start` 或 `npx expo start`）

### 操作步骤

1. 修改代码（如在 About Solo 页面添加 logo 组件）
2. 保存文件，Metro 自动重新编译
3. 在终端或真机调试菜单中 **按 `r`**
4. 真机自动 reload，立即看到最新效果

### 调试快捷键（开发服务器运行时）

```
› Press a     │ open Android
› shift+a     │ select an Android device or emulator
› Press r     │ reload app
› Press j     │ open debugger
› Press m     │ toggle menu
› shift+m     │ more tools
› Press o     │ open project code in your editor
› Press c     │ show project QR
```

### 连接真机时的典型日志
```
› Opening on Android...
› Opening exp+voice-mobile://expo-development-client/?url=http%3A%2F%2F192.168.0.146%3A8081 on PLT140
```

> 注意：此方式仅同步 JS Bundle 和资源文件变更。**如果修改了原生代码、AndroidManifest.xml、添加了原生依赖，必须重新构建 Development Client。**

---

## 3. 方式二：EAS Update（OTA 热更新）

适用于：JS/组件/图片资源变更，需要推送给已安装 App 的用户（包括开发客户端和正式版）。

### 原理
- 项目已配置 `updates.url`（`app.config.js` 中指向 Expo Update 服务）
- `runtimeVersion: { policy: "appVersion" }` 确保更新与原生运行时兼容
- 设备启动或前台恢复时自动检查并下载更新

### 操作步骤

```bash
cd app

# 推送到 development 频道（开发客户端接收）
npx eas-cli update --channel development --message "About Solo 添加 logo"

# 推送到 production 频道（正式版用户接收）
npx eas-cli update --channel production --message "About Solo 添加 logo"
```

### 验证更新
1. 确保终端显示 `Update published` 成功信息
2. 真机上杀死 App 进程后重新打开
3. 或在 App 内设置中手动触发"检查更新"
4. 更新会在后台下载，下次启动或特定时机生效（取决于 App 的更新策略配置）

### 注意事项
- EAS Update **仅支持 JS 和资源文件变更**，不支持原生代码/配置变更
- 更新受 `runtimeVersion` 限制，如果后续升级了 Expo SDK 或原生依赖，可能需要重新构建原生包
- 可通过 Expo Dashboard 查看更新发布历史和安装统计

---

## 4. 方式三：EAS Build（重新构建安装包）

适用于：原生代码变更、生成新 APK 分发、新设备安装、Development Client 重建。

### 构建配置（eas.json）

| Profile | 用途 | 输出 |
|---------|------|------|
| `development` | 开发调试客户端 | APK (Debug) |
| `production` | Google Play 上架 | AAB |
| `production-apk` | 内部分发/测试 | APK (Release) |

### 操作步骤

#### 4.1 构建开发版（Development Client）
```bash
cd app
npx eas-cli build --profile development --platform android
```
- 输出：Debug APK，支持 Development Client 协议
- 用途：本地开发、真机调试

#### 4.2 构建生产版 APK（内部分发）
```bash
cd app
npx eas-cli build --profile production-apk --platform android
```
- 输出：Release APK，可直接安装分发
- 用途：内部测试、未上架前的用户分发

#### 4.3 构建生产版 AAB（Google Play 上架）
```bash
cd app
npx eas-cli build --profile production --platform android
```
- 输出：AAB 文件，用于 Google Play Console 上传
- `versionCode` 会自动递增（`autoIncrement: "versionCode"`）

### 安装到真机

1. EAS Build 完成后会提供下载链接或二维码
2. 方式 A：直接下载 APK 到手机安装（需允许"未知来源"安装）
3. 方式 B：通过 `adb` 安装
   ```bash
   adb install -r ./path/to/app-release.apk
   ```
4. 方式 C：使用 EAS 内部分发页面，扫码直接安装

---

## 5. 方式四：本地 Gradle 构建 + adb 安装

适用于：**不想走远程 EAS 服务**，在本机直接构建 APK 并安装到真机。

### 前提条件
- Android SDK 已配置（`adb` 命令可用）
- 真机已通过 USB 连接并开启调试模式
- 项目已执行过 `prebuild`（`app/android` 目录存在）

### 操作步骤

```bash
# 1. 进入 Android 项目目录
cd app/android

# 2. 本地构建 Release APK（使用 debug signing）
./gradlew assembleRelease --no-daemon

# 3. 安装到真机
adb install -r app/build/outputs/apk/release/app-release.apk
```

### 构建输出
- APK 路径：`app/android/app/build/outputs/apk/release/app-release.apk`
- 当前 `build.gradle` 中 `release` 的 `signingConfig` 指向 `signingConfigs.debug`，因此本地可直接安装，无需额外 keystore

### 优缺点

| 优点 | 缺点 |
|------|------|
| 完全不走远程 EAS 服务 | 依赖本地 Android 开发环境 |
| 构建速度快（3-5 分钟） | 仅基于现有 prebuild 配置，无法自动切换 variant |
| 适合快速验证 Release 效果 | 正式分发仍需配置 release keystore |

> 注意：如果修改了 `app.config.js` 中的原生配置（如 packageId、权限、插件），需要先用对应 variant 重新执行 `expo prebuild`。

---

## 6. 决策流程图

```
修改了 About Solo 页面（加 logo）
           │
           ▼
┌─────────────────────┐
│ 是否只改了 JS/组件/   │
│ 图片资源？           │
└─────────────────────┘
     │           │
    是          否
     │           │
     ▼           ▼
┌──────────┐  ┌─────────────────┐
│ 本地调试？ │  │ 修改了原生代码/  │
└──────────┘  │ 原生依赖/配置？   │
  │     │     └─────────────────┘
 是     否          │
  │     │          是
  ▼     ▼           │
按 r   ┌──────────┐ ▼
reload │不走远程？ │ ► EAS Build
       └──────────┘ 远程构建 APK
        │      │
       是      否
        │      │
        ▼      ▼
   本地 Gradle  EAS Update
   assemble    OTA 推送
   + adb
```

---

## 7. 常见问题

### Q1: 按 `r` reload 后没有看到修改？
- 检查 Metro 终端是否有编译错误（红屏）
- 确认保存了文件且 Metro 已完成打包
- 尝试完全关闭 App 重新打开
- 检查是否缓存了旧 Bundle：`npx expo start --clear`

### Q2: EAS Update 推送后真机没收到？
- 确认推送的 `channel` 与设备上 App 的 channel 一致
- Development Client 使用 `development` channel，正式版使用 `production` channel
- 更新下载是异步的，可能需要重启 App 1-2 次
- 检查 `runtimeVersion` 是否匹配

### Q3: 如何查看当前 App 的版本和更新状态？
- 在 App 的 About/Settings 页面查看版本号
- 或通过代码调用 `Updates.checkForUpdateAsync()` 主动检查

### Q4: 构建失败怎么办？
- 查看 EAS 构建日志，定位具体错误
- 本地先验证：`npm run android:production` 或 `npm run android:development`
- 检查 `eas.json` 中的 gradle 命令和 env 变量配置
- 确保 monorepo 依赖已正确构建：`npm run build:workspace-deps`

---

## 8. 相关命令速查

```bash
# 进入应用目录
cd app

# 启动开发服务器
npm run start

# 本地构建并运行 Android（development debug）
npm run android:development

# 本地构建并运行 Android（production release）
npm run android:production

# EAS Update
eas update --channel <development|production> --message "描述"

# 本地 Gradle 构建 Release APK
cd app/android && ./gradlew assembleRelease --no-daemon

# adb 安装 APK
adb install -r app/android/app/build/outputs/apk/release/app-release.apk

# EAS Build
eas build --profile <development|production-apk|production> --platform android

# 查看 EAS 配置
eas config

# 查看已发布的更新
eas update:list
```

---

*文档基于项目当前配置生成，如有构建流程变更请同步更新。*
