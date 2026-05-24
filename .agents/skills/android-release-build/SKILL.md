---
name: android-release-build
description: Build and release the Solo Android app with proper logo, adaptive icon, and splash screen handling. Use when the user wants to build an Android release APK, update app icons, fix splash screen cropping, or publish to a physical Android device via adb.
---

# Android Release Build

## Overview

Build a production-ready Android release APK for the Solo Expo/React Native app, ensuring the logo renders correctly on the launcher icon (adaptive icon) and the splash screen without being clipped by the system.

## When to Use

- Building an Android release APK for local testing or distribution
- Updating the app logo and needing it to display correctly on Android
- Fixing a logo that is cropped on the launcher icon or splash screen
- Publishing the app to a physical Android device via `adb`

**When NOT to use:**

- iOS builds (use Xcode or `expo run:ios` instead)
- Web builds (use `expo export --platform web`)
- EAS Cloud builds (use `eas build` instead)

## The Process

```
PREPARE LOGO ──→ UPDATE RESOURCES ──→ BUILD ──→ INSTALL
      │               │                  │           │
      ▼               ▼                  ▼           ▼
Square image   Android native      Gradle       adb install
White bg       config & assets     assemble     & launch
Centered logo  (icons, splash)     release
```

## Step 1: Prepare the Logo Image

The source logo must be processed into **two variants** before copying to the project:

### 1a. General-Purpose Square Logo

Used for: `icon.png`, `solo-logo.png`, favicons, iOS icons, and Android fallback launcher icons.

Requirements:
- **Square aspect ratio** (1:1). If the source is not square, pad it to the largest dimension with a **white background** (`#FFFFFF`).
- Any resolution (e.g., 480×480, 1024×1024). Higher is better for store submissions.

Python snippet:

```python
from PIL import Image

src = "/path/to/logo.png"
dst = "/tmp/logo-square.png"

img = Image.open(src).convert("RGBA")
w, h = img.size
max_side = max(w, h)

canvas = Image.new("RGBA", (max_side, max_side), (255, 255, 255, 255))
x = (max_side - w) // 2
y = (max_side - h) // 2
canvas.paste(img, (x, y), img)

canvas.convert("RGB").save(dst, "PNG")
```

### 1b. Android Adaptive Icon Foreground

Used for: `android-icon-foreground.png` and `ic_launcher_foreground.webp`.

Requirements:
- **Transparent background** (not white).
- Logo must be **centered** and take up only **~60% of the canvas** (leaving ~20% padding on each side). Android adaptive icons are masked into various shapes (circle, rounded square, etc.); content near the edges will be clipped.

Python snippet:

```python
from PIL import Image

src = "/tmp/logo-square.png"
dst = "/tmp/android-icon-foreground.png"

img = Image.open(src).convert("RGBA")
size = max(img.size)
logo_size = int(size * 0.60)
padding = (size - logo_size) // 2

logo_resized = img.resize((logo_size, logo_size), Image.LANCZOS)
foreground = Image.new("RGBA", (size, size), (0, 0, 0, 0))
foreground.paste(logo_resized, (padding, padding), logo_resized)
foreground.save(dst, "PNG")
```

### 1c. Android Splash Screen Logo

Used for: `splash-icon.png` (Expo asset) and `splashscreen_logo.png` (Android native drawable).

Requirements:
- **Transparent background** for the native drawable, or **white background** for the Expo asset.
- Logo must be **centered** and take up only **~40% of the canvas** (leaving ~30% padding on each side). The Android 12+ SplashScreen API clips the icon to a circle; content outside the center ~66% safe zone is not guaranteed to be visible.

Python snippet:

```python
from PIL import Image

src = "/tmp/logo-square.png"

# Expo asset (white background)
img = Image.open(src).convert("RGBA")
size = max(img.size)
logo_size = int(size * 0.40)
padding = (size - logo_size) // 2

logo_resized = img.resize((logo_size, logo_size), Image.LANCZOS)

# White background version for Expo
white = Image.new("RGB", (size, size), (255, 255, 255))
white.paste(logo_resized, (padding, padding), logo_resized)
white.save("/tmp/splash-icon.png", "PNG")

# Transparent version for Android native
transparent = Image.new("RGBA", (size, size), (0, 0, 0, 0))
transparent.paste(logo_resized, (padding, padding), logo_resized)
transparent.save("/tmp/splashscreen_logo.png", "PNG")
```

## Step 2: Update Android Native Resources

### 2a. Adaptive Icon Background Color

In `app/app.config.js`, ensure the adaptive icon uses a **white background**:

```js
android: {
  adaptiveIcon: {
    backgroundColor: "#FFFFFF",
    foregroundImage: "./assets/images/android-icon-foreground.png",
  },
}
```

In `app/android/app/src/main/res/values/colors.xml`, update `iconBackground`:

```xml
<color name="iconBackground">#FFFFFF</color>
```

### 2b. Splash Screen Background Color

In `app/android/app/src/main/res/values/colors.xml`:

```xml
<color name="splashscreen_background">#ffffff</color>
```

In `app/android/app/src/main/res/values-night/colors.xml` (dark mode):

```xml
<color name="splashscreen_background">#ffffff</color>
```

### 2c. Splash Screen Logo (All DPIs)

Copy the prepared transparent splash logo to every Android drawable density folder:

```bash
SRC="/tmp/splashscreen_logo.png"
for dpi in hdpi mdpi xhdpi xxhdpi xxxhdpi; do
  cp "$SRC" "app/android/app/src/main/res/drawable-$dpi/splashscreen_logo.png"
done
```

### 2d. App Name

In `app/android/app/src/main/res/values/strings.xml`, set the production name:

```xml
<string name="app_name">Solo</string>
```

### 2e. Launcher Icons (WEBP)

Convert the square white-background logo to WebP and copy to all Android `mipmap-*` folders:

```bash
cwebp -q 95 /tmp/logo-square.png -o /tmp/logo-square.webp

for dpi in hdpi mdpi xhdpi xxhdpi xxxhdpi; do
  cp /tmp/logo-square.webp "app/android/app/src/main/res/mipmap-$dpi/ic_launcher.webp"
  cp /tmp/logo-square.webp "app/android/app/src/main/res/mipmap-$dpi/ic_launcher_round.webp"
done

# Foreground
convert /tmp/android-icon-foreground.png /tmp/android-icon-foreground.webp
for dpi in hdpi mdpi xhdpi xxhdpi xxxhdpi; do
  cp /tmp/android-icon-foreground.webp "app/android/app/src/main/res/mipmap-$dpi/ic_launcher_foreground.webp"
done
```

## Step 3: Build Release APK

### 3a. Prerequisites

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

### 3b. Build & Install

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

### 3c. Output Location

After a successful build, the APK is located at:

```
app/android/app/build/outputs/apk/release/app-release.apk
```

## Step 4: Verify

Check the following on the physical device:

1. **Launcher icon**: Long-press the icon — the logo should be centered with a white background and no clipped edges.
2. **App name**: Should display as **"Solo"**, not "Solo Debug".
3. **Cold-start splash screen**: Should show a white background with the logo centered and fully visible (not cropped).

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Logo clipped on launcher icon | `android-icon-foreground.png` fills the canvas | Add ~20% transparent padding around the logo |
| Logo clipped on splash screen | `splashscreen_logo.png` fills the canvas | Add ~30% transparent padding; use only ~40% of canvas for the logo |
| Black splash background | `splashscreen_background` is `#000000` | Change `colors.xml` to `#ffffff` |
| App shows "Solo Debug" | `strings.xml` has debug variant name | Update `app_name` to "Solo" and rebuild |
| Build fails with signing error | No release keystore configured | Either use debug keystore for local testing or create a production keystore |
| `expo run:android` starts Metro | Expected — `expo-dev-client` is included in the release variant | To build a fully standalone APK, remove `expo-dev-client` or use EAS Build |

## Checklist

- [ ] Logo is square with white background (general purpose)
- [ ] Android foreground has transparent background with ~20% padding
- [ ] Splash screen logo has transparent background with ~30% padding
- [ ] `app.config.js` → `android.adaptiveIcon.backgroundColor` is `#FFFFFF`
- [ ] `colors.xml` → `iconBackground` is `#FFFFFF`
- [ ] `colors.xml` → `splashscreen_background` is `#ffffff`
- [ ] `colors.xml` (night) → `splashscreen_background` is `#ffffff`
- [ ] `strings.xml` → `app_name` is "Solo"
- [ ] All `drawable-*/splashscreen_logo.png` files are updated
- [ ] All `mipmap-*/ic_launcher*.webp` files are updated
- [ ] Device is connected and visible in `adb devices`
- [ ] Build completes successfully (`BUILD SUCCESSFUL`)
- [ ] APK installs and launches on the device
