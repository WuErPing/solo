---
name: android-release-build
description: Build and release the Solo Android app. Use when the user wants to build an Android release APK, install it on a physical device via adb, or verify a release build. For logo or icon changes, use the `change-android-logo` skill first.
---

# Android Release Build

## Overview

Build a production-ready Android release APK for the Solo Expo/React Native app and install it on a device via `adb`.

## When to Use

- Building an Android release APK for local testing or distribution
- Publishing the app to a physical Android device via `adb`
- Verifying a release build launches correctly

**When NOT to use:**

- Updating app logos or icons (use `change-android-logo` instead)
- iOS builds (use Xcode or `expo run:ios` instead)
- Web builds (use `expo export --platform web`)
- EAS Cloud builds (use `eas build` instead)

## The Process

```
PREREQUISITES ──→ BUILD ──→ INSTALL ──→ VERIFY
       │              │          │           │
       ▼              ▼          ▼           ▼
Android SDK    Gradle       adb install   Check icon,
& Java         assemble     & launch      name, splash
```

## Step 1: Prerequisites

Ensure the Android SDK and Java are available:

```bash
export ANDROID_HOME="$HOME/Library/Android/sdk"
export JAVA_HOME="/opt/homebrew/opt/openjdk@17"  # or your Java path
export PATH="$JAVA_HOME/bin:$ANDROID_HOME/platform-tools:$PATH"
```

Verify the device is connected:

```bash
adb devices
```

## Step 2: Build Release APK

From the `app/` directory, run:

```bash
cd app
APP_VARIANT=production npx expo run:android --variant=release
```

This will:
1. Start Metro Bundler
2. Assemble the release APK via Gradle (`./gradlew assembleRelease`)
3. Install the APK to the connected device via `adb install`
4. Launch the app

The release build in this project uses the **debug signing config** (`android/app/build.gradle` → `signingConfig signingConfigs.debug`) for local testing. For Play Store distribution, configure a production keystore.

### Output Location

After a successful build, the APK is located at:

```
app/android/app/build/outputs/apk/release/app-release.apk
```

## Step 3: Verify

Check the following on the physical device:

1. **Launcher icon**: Long-press the icon — the logo should be centered with a white background and no clipped edges.
2. **App name**: Should display as **"Solo"**, not "Solo Debug".
3. **Cold-start splash screen**: Should show a white background with the logo centered and fully visible (not cropped).

If any visual issue is found, use the `change-android-logo` skill to update the corresponding assets and rebuild.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Build fails with signing error | No release keystore configured | Either use debug keystore for local testing or create a production keystore |
| `expo run:android` starts Metro | Expected — `expo-dev-client` is included in the release variant | To build a fully standalone APK, remove `expo-dev-client` or use EAS Build |
| Logo clipped on launcher icon | `android-icon-foreground.png` fills the canvas | Use `change-android-logo` skill to add ~20% transparent padding |
| Logo clipped on splash screen | `splashscreen_logo.png` fills the canvas | Use `change-android-logo` skill to add ~30% transparent padding |
| Black splash background | `splashscreen_background` is `#000000` | Use `change-android-logo` skill to change `colors.xml` to `#ffffff` |
| App shows "Solo Debug" | `strings.xml` has debug variant name | Use `change-android-logo` skill to update `app_name` to "Solo" and rebuild |

## Checklist

- [ ] `ANDROID_HOME` and `JAVA_HOME` are set
- [ ] Device is connected and visible in `adb devices`
- [ ] Build completes successfully (`BUILD SUCCESSFUL`)
- [ ] APK installs and launches on the device
- [ ] Launcher icon displays correctly (centered, not clipped)
- [ ] App name shows as "Solo"
- [ ] Splash screen shows white background with centered logo
