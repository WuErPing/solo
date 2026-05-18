---
name: update-project-logo
description: Batch replace all project logos, icons, and brand images with a new source image. Use when the user wants to update app icons, favicons, splash screens, notification icons, or any project-wide logo assets. Covers Expo/React Native apps, web favicons, and mask images.
---

# Update Project Logo

## Overview

Replace every logo, icon, and brand image in the project with a new source image. This skill discovers all relevant image assets (app icons, favicons, splash screens, notification icons, masks, and their variants), copies the new image to each location, and reports what was changed and what was left untouched.

## When to Use

- The user provides a new logo/image and says "update all logos" or "replace all icons"
- Rebranding or refreshing the app's visual identity
- Updating app store icons, splash screens, or adaptive icons
- Replacing favicons and their dark/light/attention/running variants

**When NOT to use:**

- Only a single image needs changing (just copy the file directly)
- The request is about code-level SVG components, not image files (use component refactoring instead)
- The user wants to edit or resize the image, not replace files (use image processing tools)

## The Process

```
SCAN ──→ CLASSIFY ──→ REPLACE ──→ VERIFY
  │           │            │            │
  ▼           ▼            ▼            ▼
Find all   Identify     Copy new     Confirm
images     categories   source to    replacements
           (icon,       each target  and flag
           favicon,     location     exceptions
           splash,
           mask...)
```

### Step 1: Scan for All Images

Search the entire project for image files, excluding `node_modules`, `.git`, and `.expo` cache directories.

```bash
# Find all image files
find <project-root> -type f \
  \( -iname "*.png" -o -iname "*.svg" -o -iname "*.ico" \
     -o -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.webp" \) \
  ! -path "*/node_modules/*" ! -path "*/.git/*" ! -path "*/.expo/*"
```

Filter for logo/brand-related files using name patterns:
- `*solo*`, `*logo*`, `*icon*`, `*favicon*`, `*splash*`, `*notification*`, `*mask*`, `*android*foreground*`

### Step 2: Classify Targets

Read `app.config.js` / `app.json` / `package.json` to understand which images serve which purpose. Typical categories in an Expo/React Native project:

| Category | Config Key / Typical Path | Notes |
|----------|---------------------------|-------|
| App Icon | `expo.icon` → `assets/images/icon.png` | Usually 1024×1024 |
| Splash Icon | `expo-splash-screen` plugin → `assets/images/splash-icon.png` | Usually 200×200 |
| Favicon | `expo.web.favicon` → `assets/images/favicon.png` | Usually 48×48 |
| Notification Icon | `expo-notifications` plugin → `assets/images/notification-icon.png` | Usually 96×96, monochrome preferred |
| Android Foreground | `expo.android.adaptiveIcon.foregroundImage` → `assets/images/android-icon-foreground.png` | Usually 1024×1024, often grayscale+alpha |
| Logo Asset | Component import → `assets/images/solo-logo.png` | Referenced by React components |
| Logo Mask | Web mask → `assets/images/solo-logo-mask.png`, `public/solo-logo-mask.png` | Used for CSS masking |
| Favicon Variants | `favicon-dark.png`, `favicon-light-*.png`, etc. | Dark/light/attention/running states |

Also scan for inline SVG components that embed the logo (e.g., `components/icons/solo-logo.tsx`). If they reference an image file, they will pick up the new image automatically. If they contain hardcoded SVG paths, they need separate code-level changes.

### Step 3: Replace

Copy the user-provided source image to every classified target location:

```bash
cp <source-image> <target-path-1>
cp <source-image> <target-path-2>
# ... repeat for every identified target
```

**Do NOT replace** third-party service icons (e.g., `claude.svg`, `codex.svg`, editor-app icons) unless the user explicitly asks.

**Do NOT replace** `.expo/cache` files — they regenerate on the next build.

### Step 4: Verify and Report

1. List all modified files with `ls -la` to confirm sizes match the source.
2. Search the codebase for any remaining hardcoded logo references (SVG components, CSS `mask-image` URLs, etc.).
3. Summarize:
   - What was replaced (list each file)
   - What was left untouched and why
   - Any warnings about size/aspect-ratio mismatches or transparency requirements

## Warnings and Best Practices

| Issue | Guidance |
|-------|----------|
| Aspect ratio mismatch | If the new image is not square but `icon.png` / `android-icon-foreground.png` must be, Expo may letterbox it. Warn the user to verify the final build. |
| Transparency | Android adaptive icons and notification icons often require transparent backgrounds. If the new image is opaque, flag this for the user. |
| Cache invalidation | `.expo/web/cache` and build caches may still contain old favicons. Recommend a clean build (`npx expo prebuild --clean` or deleting `.expo`). |
| SVG components | If a component like `SoloLogo.tsx` inlines SVG paths instead of using an `Image` tag, replacing PNG files won't affect it. Check component source and update code if needed. |
| Grayscale masks | `solo-logo-mask.png` may be expected to be a grayscale mask. Replacing it with a full-color PNG can break CSS `mask-image` effects. Verify mask usage in code before copying. |

## Verification Checklist

After completing the replacement:

- [ ] All `*logo*`, `*icon*`, `*favicon*`, `*splash*`, `*notification*`, `*mask*` image files are identified
- [ ] `app.config.js` / `app.json` references are cross-checked against actual files
- [ ] Source image is copied to every target location
- [ ] Third-party icons (Claude, Codex, editor apps, etc.) are explicitly excluded unless requested
- [ ] `.expo` cache and `node_modules` are left untouched
- [ ] Any inline SVG component embedding the logo is inspected and noted
- [ ] User is warned about aspect-ratio, transparency, or grayscale mismatches
- [ ] A clean build is recommended to invalidate caches
