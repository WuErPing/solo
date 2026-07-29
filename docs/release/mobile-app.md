# Mobile App 发版（app）

> App 基于 Expo / React Native，云构建走 EAS（Expo Application Services），也可本地构建 Android APK。
> app 版本（`app/package.json`）是整个仓库 release tag 的锚点（见 [versioning.md](versioning.md)）。

## 构建配置

- EAS profile 定义在 [`app/eas.json`](../../app/eas.json)；
- 应用变体（名称 / bundleId / Firebase 密钥）由 `APP_VARIANT` 驱动，见 [`app/app.config.js`](../../app/app.config.js)；
- `cli.appVersionSource: "remote"`：版本号由 EAS 远端管理，`production` 的 Android `versionCode` 自增。

| Profile | 用途 | 关键点 |
|---------|------|--------|
| `development` | 开发客户端 | `developmentClient`、internal 分发、`APP_VARIANT=development`、Android `assembleDebug` |
| `production` | 正式发布 | `APP_VARIANT=production`、Android `versionCode` 自增 |
| `production-apk` | 内部分发 APK | 继承 production、internal、APK buildType（跳过 lint） |

## EAS 云构建

```bash
cd app

# 正式构建（iOS + Android）
eas build --profile production

# 仅 Android APK（内部分发）
eas build --profile production-apk --platform android

# 构建完成后提交商店
eas submit --profile production
```

`submit.production` 预设：iOS `ascAppId=6758887924`；Android track `production`、`releaseStatus=completed`。

## 本地 Android APK

本地构建 release APK 并 `adb` 安装验证，使用 [`android-release-build` skill](../../.agents/skills/android-release-build/SKILL.md)（`expo run:android --variant=release`，产物在 `app/android/app/build/outputs/apk/release/app-release.apk`）。EAS 云构建仍走上面的 `eas build`。

## 验证

- EAS 构建完成后在 Expo 控制台确认产物与 `versionCode`；
- 安装后检查「设置 / 关于」页版本号（按 AGENTS.md 约定，Android 版本含 `-{date-time}` 后缀）。
