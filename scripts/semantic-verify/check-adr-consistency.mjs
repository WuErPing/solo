#!/usr/bin/env node
/**
 * ADR consistency semantic checker (PADD semantic-validation layer).
 *
 * Evaluates whether a git diff complies with every accepted ADR in
 * docs/decisions/, using an LLM as a first-pass reviewer. Results are
 * advisory by design: the script exits 0 unless --fail-on-violation is
 * passed, and any evaluator error degrades to an "uncertain" verdict
 * instead of failing the run.
 *
 * Usage:
 *   node scripts/semantic-verify/check-adr-consistency.mjs [options]
 *
 * Options:
 *   --base <ref>            Git ref to diff against (default: origin/main)
 *   --adr-dir <dir>         Directory containing adr-*.md files (default: docs/decisions)
 *   --out <file>            Write a Markdown report to this file
 *   --json <file>           Write raw JSON results to this file
 *   --max-diff-chars <n>    Truncate the diff beyond this size (default: 120000)
 *   --fail-on-violation     Exit 1 when any ADR is judged "violation"
 *   --dry-run               Print the prompts without calling the LLM
 *
 * Environment:
 *   LLM_API_KEY   (required unless --dry-run)  API key for the chat endpoint
 *   LLM_BASE_URL  (optional) OpenAI-compatible base URL, default https://api.openai.com/v1
 *   LLM_MODEL     (optional) model name, default gpt-4o-mini
 */
import { execFileSync } from 'node:child_process';
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

const args = parseArgs(process.argv.slice(2));
const BASE = args.base ?? 'origin/main';
const ADR_DIR = args['adr-dir'] ?? 'docs/decisions';
const MAX_DIFF_CHARS = Number(args['max-diff-chars'] ?? 120000);
const DRY_RUN = Boolean(args['dry-run']);
const FAIL_ON_VIOLATION = Boolean(args['fail-on-violation']);

const LLM_API_KEY = process.env.LLM_API_KEY ?? '';
const LLM_BASE_URL = (process.env.LLM_BASE_URL ?? 'https://api.openai.com/v1').replace(/\/$/, '');
const LLM_MODEL = process.env.LLM_MODEL ?? 'gpt-4o-mini';

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith('--')) continue;
    const key = a.slice(2);
    if (key === 'dry-run' || key === 'fail-on-violation') {
      out[key] = true;
    } else {
      out[key] = argv[++i];
    }
  }
  return out;
}

function collectAdrs(dir) {
  let files;
  try {
    files = readdirSync(dir).filter((f) => /^adr-.*\.md$/i.test(f));
  } catch {
    return [];
  }
  const adrs = [];
  for (const f of files.sort()) {
    const full = path.join(dir, f);
    const content = readFileSync(full, 'utf8');
    const status = parseStatus(content);
    // Only enforce accepted decisions; proposed/deprecated/superseded ADRs
    // carry no binding constraints.
    if (status && !/^accepted$/i.test(status)) continue;
    adrs.push({ file: full, status: status ?? 'unknown', content });
  }
  return adrs;
}

function parseStatus(content) {
  const m = content.match(/\|\s*\*\*Status\*\*\s*\|\s*([^|]+)\|/i);
  return m ? m[1].trim() : null;
}

function getDiff(base) {
  const excludes = [
    ':(exclude)*.lock',
    ':(exclude)*.sum',
    ':(exclude)package-lock.json',
    ':(exclude)app-bridge/src/generated/**',
    ':(exclude)**/coverage/**',
    ':(exclude)**/dist/**',
  ];
  let diff;
  try {
    diff = execFileSync('git', ['diff', '--unified=3', `${base}...HEAD`, '--', '.', ...excludes], {
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
    });
  } catch (err) {
    throw new Error(`Failed to compute diff against "${base}": ${err.message}`);
  }
  let truncated = false;
  if (diff.length > MAX_DIFF_CHARS) {
    diff = diff.slice(0, MAX_DIFF_CHARS);
    truncated = true;
  }
  return { diff, truncated };
}

function buildPrompt(adr, diff, truncated) {
  const system = [
    'You are an architecture-conformance evaluator for the Solo project.',
    'You decide whether a code change complies with one Architecture Decision Record (ADR).',
    'Judge ONLY constraints the ADR actually states or directly implies; do not invent new rules.',
    'A change unrelated to the ADR scope is "compliant".',
    'Use "uncertain" when the diff lacks the context needed to decide.',
    'Respond with STRICT JSON matching this schema, nothing else:',
    '{',
    '  "verdict": "compliant" | "violation" | "uncertain",',
    '  "violations": [{ "file": "<path or empty>", "summary": "<one sentence>" }],',
    '  "rationale": "<2-4 sentences citing the ADR constraint and the diff evidence>",',
    '  "confidence": <number between 0 and 1>',
    '}',
  ].join('\n');

  const user = [
    `# ADR under review: ${adr.file}`,
    '',
    adr.content,
    '',
    '---',
    '',
    `# Code change under review (git diff vs base)${truncated ? ' [TRUNCATED]' : ''}`,
    '',
    diff.length > 0 ? diff : '(empty diff)',
  ].join('\n');

  return { system, user };
}

async function evaluate(adr, diff, truncated) {
  const { system, user } = buildPrompt(adr, diff, truncated);
  if (DRY_RUN) {
    return { dryRun: true, system, user };
  }
  const res = await fetch(`${LLM_BASE_URL}/chat/completions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${LLM_API_KEY}`,
    },
    body: JSON.stringify({
      model: LLM_MODEL,
      temperature: 0,
      response_format: { type: 'json_object' },
      messages: [
        { role: 'system', content: system },
        { role: 'user', content: user },
      ],
    }),
  });
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new Error(`LLM request failed: HTTP ${res.status} ${body.slice(0, 300)}`);
  }
  const data = await res.json();
  const text = data?.choices?.[0]?.message?.content ?? '';
  return normalizeResult(JSON.parse(text));
}

function normalizeResult(raw) {
  const verdicts = ['compliant', 'violation', 'uncertain'];
  return {
    verdict: verdicts.includes(raw?.verdict) ? raw.verdict : 'uncertain',
    violations: Array.isArray(raw?.violations)
      ? raw.violations.map((v) => ({ file: String(v?.file ?? ''), summary: String(v?.summary ?? '') }))
      : [],
    rationale: String(raw?.rationale ?? ''),
    confidence: typeof raw?.confidence === 'number' ? Math.min(1, Math.max(0, raw.confidence)) : null,
  };
}

function renderMarkdown(results, meta) {
  const icon = { compliant: '✅', violation: '❌', uncertain: '⚠️' };
  const lines = [
    '<!-- solo-adr-consistency-report -->',
    '## ADR Consistency Report (semantic check)',
    '',
    `- Base: \`${meta.base}\` · Model: \`${meta.model}\` · ADRs evaluated: ${results.length}` +
      (meta.truncated ? ' · ⚠️ diff truncated' : ''),
    '',
    '| ADR | Verdict | Confidence | Rationale |',
    '|-----|---------|-----------|-----------|',
  ];
  for (const r of results) {
    const conf = r.result.confidence == null ? '—' : r.result.confidence.toFixed(2);
    const rationale = (r.result.rationale || r.error || '').replace(/\|/g, '\\|').replace(/\n/g, ' ');
    lines.push(`| \`${path.basename(r.file)}\` | ${icon[r.result.verdict] ?? '⚠️'} ${r.result.verdict} | ${conf} | ${rationale} |`);
  }
  const details = results.flatMap((r) =>
    r.result.violations.map((v) => `- **${path.basename(r.file)}** \`${v.file || '?'}\`: ${v.summary}`),
  );
  if (details.length > 0) {
    lines.push('', '### Violations', '', ...details);
  }
  lines.push(
    '',
    '> Advisory first-pass review by an LLM evaluator (PADD semantic-validation layer).',
    '> It does not block merging; a human reviewer makes the final call.',
  );
  return lines.join('\n');
}

async function main() {
  const adrs = collectAdrs(ADR_DIR);
  if (adrs.length === 0) {
    console.log(`No accepted ADRs found in ${ADR_DIR}; nothing to check.`);
    return;
  }
  const { diff, truncated } = getDiff(BASE);
  console.log(`Evaluating ${adrs.length} ADR(s) against diff vs ${BASE} (${diff.length} chars${truncated ? ', truncated' : ''})`);

  if (DRY_RUN) {
    for (const adr of adrs) {
      const { system, user } = buildPrompt(adr, diff, truncated);
      console.log(`\n=== DRY RUN prompt for ${adr.file} ===\n[system]\n${system}\n[user, first 500 chars]\n${user.slice(0, 500)}\n...`);
    }
    return;
  }
  if (!LLM_API_KEY) {
    console.error('ERROR: LLM_API_KEY is not set (use --dry-run to inspect prompts without it).');
    process.exit(2);
  }

  const results = [];
  for (const adr of adrs) {
    try {
      const result = await evaluate(adr, diff, truncated);
      results.push({ file: adr.file, result });
      console.log(`  ${adr.file}: ${result.verdict}`);
    } catch (err) {
      // Evaluator failures degrade to "uncertain", never to a hard failure.
      results.push({
        file: adr.file,
        error: err.message,
        result: { verdict: 'uncertain', violations: [], rationale: `Evaluator error: ${err.message}`, confidence: null },
      });
      console.log(`  ${adr.file}: uncertain (evaluator error: ${err.message})`);
    }
  }

  const meta = { base: BASE, model: LLM_MODEL, truncated };
  const report = renderMarkdown(results, meta);
  if (args.out) writeFileSync(args.out, report + '\n');
  if (args.json) writeFileSync(args.json, JSON.stringify({ meta, results }, null, 2) + '\n');
  console.log('\n' + report);

  const hasViolation = results.some((r) => r.result.verdict === 'violation');
  if (FAIL_ON_VIOLATION && hasViolation) {
    console.error('\nFAIL: ADR violation(s) detected.');
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(`ERROR: ${err.message}`);
  process.exit(2);
});
