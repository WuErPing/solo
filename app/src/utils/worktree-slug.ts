import { createUniqueNameId } from "mnemonic-id";

const MAX_SLUG_LENGTH = 40;

export interface GenerateWorktreeSlugInput {
  /** User-provided prompt / first message for the workspace. */
  prompt?: string;
  /** Title of a selected GitHub PR, used when no prompt is available. */
  prTitle?: string;
  /** Display name of the project (e.g. "Solo"). */
  projectDisplayName?: string;
  /** Source directory path for the project. */
  sourceDirectory?: string;
}

/**
 * Generates a URL-safe worktree directory / branch name slug.
 *
 * Priority:
 * 1. slugified user prompt
 * 2. slugified PR title
 * 3. slugified project display name
 * 4. slugified source directory basename
 * 5. unique mnemonic fallback
 */
export function generateWorktreeSlug(input: GenerateWorktreeSlugInput): string {
  const fromPrompt = slugify(input.prompt);
  if (fromPrompt) {
    return fromPrompt;
  }

  const fromPr = slugify(input.prTitle);
  if (fromPr) {
    return fromPr;
  }

  const fromProject = slugify(input.projectDisplayName);
  if (fromProject) {
    return fromProject;
  }

  const fromDir = slugify(input.sourceDirectory?.split(/[\\/]/).pop());
  if (fromDir) {
    return fromDir;
  }

  return createUniqueNameId();
}

function slugify(value: string | null | undefined): string | null {
  const trimmed = value?.trim();
  if (!trimmed) {
    return null;
  }

  let slug = trimmed.toLowerCase();
  slug = slug.replace(/[^a-z0-9]+/g, "-");
  slug = slug.replace(/^-+|-+$/g, "");
  if (!slug) {
    return null;
  }

  if (slug.length > MAX_SLUG_LENGTH) {
    const truncated = slug.slice(0, MAX_SLUG_LENGTH);
    const lastHyphen = truncated.lastIndexOf("-");
    if (lastHyphen > MAX_SLUG_LENGTH / 2) {
      slug = truncated.slice(0, lastHyphen);
    } else {
      slug = truncated.replace(/-+$/, "");
    }
  }

  return slug;
}
