---
name: change-android-logo
description: Update the Android app logo, adaptive icon, and splash screen assets for the Solo Expo/React Native app. Use when the user wants to change the app icon, fix logo cropping on the launcher icon or splash screen, or update Android-native icon and splash resources.
---

# Change Android Logo

## Overview

Prepare and install logo assets for the Android app, ensuring the logo renders correctly on the launcher adaptive icon and the splash screen without being clipped by the system.

## When to Use

- Updating the app logo and needing it to display correctly on Android
- Fixing a logo that is cropped on the launcher icon or splash screen
- Updating Android-native icon, splash, or mipmap resources

**When NOT to use:**

- iOS icon or splash changes (use Xcode asset catalogs or `expo prebuild` instead)
- Web favicon changes (update `assets/images/favicon.png` and `app.config.js` directly)
- Batch replacing all project logos (use `update-project-logo` instead)
- Building or installing the APK (use `android-release-build` instead)

## The Process

```
PREPARE LOGO ──→ UPDATE RESOURCES ──→ VERIFY
      │               │                    |
      ▼               ▼                    ▼
Square image   Android native         Check on
White bg       config & assets        device or
Centered logo  (icons, splash)        emulator
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

### 2f. Expo Asset Images

Copy the prepared square logo and splash icon to the Expo assets folder:

```bash
cp /tmp/logo-square.png app/assets/images/icon.png
cp /tmp/logo-square.png app/assets/images/solo-logo.png
cp /tmp/splash-icon.png app/assets/images/splash-icon.png
cp /tmp/android-icon-foreground.png app/assets/images/android-icon-foreground.png
```

## Step 3: Verify

After updating resources, verify the changes visually by building and installing the app (see `android-release-build` skill), or inspect the files directly:

1. Check that all `drawable-*/splashscreen_logo.png` files are updated and have the expected dimensions.
2. Check that all `mipmap-*/ic_launcher*.webp` files are updated.
3. Confirm `colors.xml` and `strings.xml` values are correct.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Logo clipped on launcher icon | `android-icon-foreground.png` fills the canvas | Add ~20% transparent padding around the logo (Step 1b) |
| Logo clipped on splash screen | `splashscreen_logo.png` fills the canvas | Add ~30% transparent padding; use only ~40% of canvas for the logo (Step 1c) |
| Black splash background | `splashscreen_background` is `#000000` | Change `colors.xml` to `#ffffff` (Step 2b) |
| App shows "Solo Debug" | `strings.xml` has debug variant name | Update `app_name` to "Solo" and rebuild (Step 2d) |

## Checklist

- [ ] Logo is square with white background (general purpose, Step 1a)
- [ ] Android foreground has transparent background with ~20% padding (Step 1b)
- [ ] Splash screen logo has transparent background with ~30% padding (Step 1c)
- [ ] `app.config.js` → `android.adaptiveIcon.backgroundColor` is `#FFFFFF` (Step 2a)
- [ ] `colors.xml` → `iconBackground` is `#FFFFFF` (Step 2a)
- [ ] `colors.xml` → `splashscreen_background` is `#ffffff` (Step 2b)
- [ ] `colors.xml` (night) → `splashscreen_background` is `#ffffff` (Step 2b)
- [ ] `strings.xml` → `app_name` is "Solo" (Step 2d)
- [ ] All `drawable-*/splashscreen_logo.png` files are updated (Step 2c)
- [ ] All `mipmap-*/ic_launcher*.webp` files are updated (Step 2e)
- [ ] Expo asset images (`icon.png`, `solo-logo.png`, `splash-icon.png`, `android-icon-foreground.png`) are updated (Step 2f)
