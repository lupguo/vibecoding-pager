# SessionCard 紧凑展开布局 + BrushCleaning 图标 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 SessionCard 展开态从 "4 元信息块 + 代码块 + 折叠区 + 切换按钮" 简化为 "代码块 + PATH/SESSION footer"，并把项目清理图标从 `Trash2` 换成 `BrushCleaning`。

**Architecture:** 纯前端改动。SessionCard 保留折叠态完全不变；展开态删除 4 行元信息（TIME / TOOL ID / TTY / PERM）+「更多/收起」按钮 + 相关 state；`<pre>` 代码块上移成视觉焦点；剩余 PATH/SESSION 转为 dashed-border footer 单行 horizontal 显示。MetaRow 组件新增 `inline?: boolean` prop 支持 footer 简化样式。SessionList 中的 `Trash2` 替换为 `BrushCleaning`（lucide v1.17.0 没有 Broom，BrushCleaning 是官方"清理"语义图标）。

**Tech Stack:** React 18, TypeScript, Tailwind utility classes, lucide-react icons. **No frontend test framework** — every task ends with a manual visual verification via `make dev`, and the final task runs the project's regression checklist (`docs/regression/2026-06-01-ui-state-regression.md`).

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `frontend/src/components/SessionList.tsx` | Project group rendering + per-project clear button | Swap `Trash2` import & usage to `BrushCleaning` (1-line change in 2 spots) |
| `frontend/src/components/SessionCard.tsx` | Single session card, collapsed + expanded layout | Delete `showMore` state, 4 metadata rows, "更多" button, `formatTimestamp`, unused icon imports. Reorder expanded section to put `<pre>` first, then footer. Tighten spacing (`pl-[22px]` → `pl-[18px]`). Add `inline` prop usage on `MetaRow` for footer rows. |
| `frontend/src/components/SessionCard.tsx` (MetaRow component) | Single-row metadata renderer | Add optional `inline?: boolean` prop; when true, render horizontally without the uppercase label text, with hover-revealed Copy button |
| `docs/regression/2026-06-01-ui-state-regression.md` | Manual UI regression checklist | Update §B.5 (Trash2 → BrushCleaning) and rewrite §C (Expanded Card) to reflect new structure |

---

## Branching

Work on a feature branch off `main`:

```bash
git checkout -b feature/sessioncard-compact-layout-2026-06-02
```

Each Task ends with its own commit. Final merge into `main` after all tasks pass and regression checklist is run.

---

## Pre-flight check

- [ ] **Step 1: Confirm clean working tree on main**

Run:
```bash
cd /private/data/projects/github.com/sapaude/pager
git status --short
git rev-parse --abbrev-ref HEAD
```

Expected: working tree clean (or only `??` untracked files), branch is `main`. If anything else is staged or modified, stop and ask.

- [ ] **Step 2: Create feature branch**

Run:
```bash
git checkout -b feature/sessioncard-compact-layout-2026-06-02
```

Expected: `Switched to a new branch 'feature/sessioncard-compact-layout-2026-06-02'`

- [ ] **Step 3: Confirm baseline lints clean**

Run:
```bash
make lint
```

Expected: PASS (`go vet` then `cd frontend && npx tsc --noEmit` both exit 0).

---

## Task 1: Swap Trash2 → BrushCleaning in SessionList

**Why first:** Lowest-risk change (one file, two single-line edits). Lets you visually confirm the new icon before touching SessionCard structure.

**Files:**
- Modify: `frontend/src/components/SessionList.tsx:2` (import) and `:73` (usage)

- [ ] **Step 1: Update lucide-react import**

Open `frontend/src/components/SessionList.tsx`. Replace line 2:

```tsx
import { ChevronDown, ChevronRight, Folder, Trash2 } from 'lucide-react'
```

with:

```tsx
import { BrushCleaning, ChevronDown, ChevronRight, Folder } from 'lucide-react'
```

(Imports must stay alphabetical to match the project's existing convention — verify by glancing at the alphabetical order in this file's other imports.)

- [ ] **Step 2: Update icon usage**

In the same file, replace line 73:

```tsx
<Trash2 size={11} strokeWidth={2} />
```

with:

```tsx
<BrushCleaning size={11} strokeWidth={2} />
```

`size={11}` and `strokeWidth={2}` are intentionally unchanged so surrounding layout is identical.

- [ ] **Step 3: TypeScript check**

Run:
```bash
make lint
```

Expected: PASS. If TS complains about `Trash2` still being referenced anywhere else, grep:
```bash
grep -rn 'Trash2' frontend/src
```
Expected: zero hits.

- [ ] **Step 4: Visual smoke test**

Run:
```bash
make dev
```

In the running app:
1. Hover over any project group header in the session list.
2. Confirm the icon on the right side of the project header is now a brush-with-dustpan shape instead of a trash can.
3. Click it — the cards in that project should still vanish (functionality unchanged).
4. Stop the dev server (Ctrl+C in the terminal that ran `make dev`).

If the icon doesn't render or looks wrong, check the browser DevTools console for import errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/SessionList.tsx
git -c commit.gpgsign=false commit -m "feat(ui): swap project clear icon Trash2 → BrushCleaning

The per-project clear action sweeps cards out of the list rather than
tossing them in the trash, so a broom-style icon reads more accurately.
lucide v1.17.0 has no Broom but ships BrushCleaning (a brush over a
dustpan), which lucide explicitly designed for cleanup actions.

Same size (11) and strokeWidth (2) as before — no surrounding layout
change."
```

---

## Task 2: Add `inline` prop to MetaRow

**Why before deleting rows:** We need the new footer rendering path ready before we rewire SessionCard to use it. Adding the prop with a `false` default keeps existing `MetaRow` callers (PATH/SESSION/TIME and TOOL ID/TTY/PERM) rendering exactly as today, so this task is a pure additive change with zero visual impact until Task 3 starts using `inline={true}`.

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx:41-80` (MetaRow component)

- [ ] **Step 1: Update MetaRow signature and add inline branch**

In `frontend/src/components/SessionCard.tsx`, replace the entire `MetaRow` function (lines 41-80) with:

```tsx
function MetaRow({
  Icon, label, value, copyable, inline,
}: {
  Icon: typeof FolderOpen
  label: string
  value: string
  copyable?: boolean
  /**
   * When true, render in a horizontal "footer" style: no uppercase label
   * text (the icon carries the meaning), tighter padding, Copy button only
   * appears when the parent row is hovered. Used in the compact expanded
   * card footer; existing default-block callers leave this unset.
   */
  inline?: boolean
}) {
  const [copied, setCopied] = useState(false)
  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!value) return
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), COPY_FEEDBACK_MS)
    } catch (err) {
      console.error('[pager] copy failed:', err)
    }
  }

  if (inline) {
    return (
      <div
        className="group/metarow flex items-center gap-[5px] text-[10px] text-[--pager-text-muted] min-w-0"
        title={label}>
        <Icon size={11} className="opacity-65 shrink-0" strokeWidth={2} />
        <span
          className="font-mono text-[--pager-text-secondary] truncate min-w-0"
          title={value}>
          {value || '—'}
        </span>
        <button
          onClick={handleCopy}
          className={`w-[16px] h-[16px] flex items-center justify-center rounded-[3px] text-[--pager-text-faint] hover:bg-[rgba(255,255,255,0.06)] hover:text-[--pager-text-secondary] shrink-0 transition-opacity opacity-0 group-hover/metarow:opacity-100 ${
            !copyable || !value ? 'invisible' : ''
          }`}
          title="复制">
          {copied ? <Check size={10} strokeWidth={2.5} /> : <Copy size={10} strokeWidth={2} />}
        </button>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-[6px] px-[8px] py-[3px] text-[10px] leading-[1.6] text-[--pager-text-secondary]">
      <Icon size={11} className="opacity-65 shrink-0 text-[--pager-text-muted]" strokeWidth={2} />
      <span className="text-[9px] uppercase tracking-[0.3px] text-[--pager-text-muted] w-[60px] shrink-0">
        {label}
      </span>
      <span className="font-mono text-[--pager-text-primary] truncate flex-1" title={value}>
        {value || '—'}
      </span>
      <button
        onClick={handleCopy}
        className={`w-[18px] h-[18px] flex items-center justify-center rounded-[3px] text-[--pager-text-faint] hover:bg-[rgba(255,255,255,0.06)] hover:text-[--pager-text-secondary] shrink-0 transition-colors ${
          !copyable || !value ? 'invisible' : ''
        }`}
        title="复制">
        {copied ? <Check size={11} strokeWidth={2.5} /> : <Copy size={11} strokeWidth={2} />}
      </button>
    </div>
  )
}
```

Key points to verify in the diff:
- The non-inline branch (the trailing `return (...)`) is byte-identical to lines 61-79 in the original — we only **prefixed** an `if (inline) { ... }` block.
- The `group/metarow` Tailwind named-group syntax requires Tailwind ≥ 3.2 (verify by checking `frontend/package.json` if unsure — confirmed present in this repo).
- Inline mode uses `size={10}` for Copy/Check icons (vs `11` in non-inline) and `w-[16px] h-[16px]` button (vs `w-[18px] h-[18px]`) — keeps the footer visually lighter.

- [ ] **Step 2: TypeScript check**

```bash
make lint
```

Expected: PASS. The new prop is optional, so existing callers continue to compile.

- [ ] **Step 3: Visual no-regression check**

```bash
make dev
```

In the app, click any session card to expand it. Confirm:
- PATH / SESSION / TIME rows render *exactly as before* (with full uppercase labels, regular Copy buttons always visible).
- "更多" button still works, and TOOL ID / TTY / PERM rows still render in their old style.

This proves the additive change didn't break the existing rendering path. Stop dev server.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/SessionCard.tsx
git -c commit.gpgsign=false commit -m "feat(ui): add inline mode to MetaRow

Inline mode is a horizontal compact rendering used by the upcoming
SessionCard expanded-card footer: icon + value + (hover-revealed) Copy
button, no uppercase label text (the icon carries semantics at footer
font sizes).

Existing block-mode callers (PATH/SESSION/TIME and TOOL ID/TTY/PERM
inside the expanded metadata blocks) leave the prop unset and continue
to render unchanged. Pure additive change — no behavior shift in this
commit."
```

---

## Task 3: Compact SessionCard expanded section

**Goal:** Replace the four-block expanded section (PATH/SESSION/TIME → `<pre>` → optional TOOL ID/TTY/PERM → "更多" toggle) with the spec's two-block design (`<pre>` first, then PATH/SESSION inline footer).

**Files:**
- Modify: `frontend/src/components/SessionCard.tsx` (multiple regions)

- [ ] **Step 1: Trim icon imports**

Replace lines 2-6 of `frontend/src/components/SessionCard.tsx`:

```tsx
import {
  ArrowRight, AlertTriangle, CheckCircle2, ChevronDown, ChevronUp,
  Clock, Copy, Check, FolderOpen, HandHelping, Hash, Loader2,
  ShieldCheck, Terminal, Wrench,
} from 'lucide-react'
```

with:

```tsx
import {
  ArrowRight, AlertTriangle, CheckCircle2,
  Copy, Check, FolderOpen, HandHelping, Hash, Loader2,
} from 'lucide-react'
```

Removed: `ChevronDown`, `ChevronUp`, `Clock`, `ShieldCheck`, `Terminal`, `Wrench` (no longer used after this task).

- [ ] **Step 2: Delete formatTimestamp**

Remove lines 82-88 (the entire `formatTimestamp` function). It will be unused after Step 4 deletes its only caller.

- [ ] **Step 3: Delete showMore state**

In `SessionCard` (the default-export function), remove the `showMore` state line. The hooks block goes from:

```tsx
const [jumping, setJumping] = useState(false)
const [expanded, setExpanded] = useState(false)
const [showMore, setShowMore] = useState(false)
```

to:

```tsx
const [jumping, setJumping] = useState(false)
const [expanded, setExpanded] = useState(false)
```

- [ ] **Step 4: Replace the expanded section**

Find the block that starts with `{expanded && (` (currently line 158) and ends at its matching `)}` (currently line 201). Replace the **entire** block with:

```tsx
{expanded && (
  <div className="mt-[6px] pl-[18px]">
    {session.LastEvent?.content_raw && (
      <pre className="px-[6px] py-[5px] bg-[--pager-detail-bg] border border-[--pager-detail-border] rounded-[6px] text-[10px] leading-relaxed text-[--pager-detail-text] font-mono whitespace-pre-wrap break-all max-h-32 overflow-y-auto">
        {session.LastEvent.content_raw}
      </pre>
    )}
    <div className="flex items-center gap-[14px] mt-[5px] pt-[5px] border-t border-dashed border-[rgba(255,255,255,0.08)] min-w-0">
      <MetaRow Icon={FolderOpen} label="PATH" value={session.CWD || ''} copyable inline />
      <MetaRow Icon={Hash} label="SESSION" value={session.SessionID || session.Key || ''} copyable inline />
    </div>
  </div>
)}
```

Key differences from the original:
- Outer `<div>` uses `pl-[18px]` (was `pl-[22px]`).
- The `<pre>` is now the **first** child of the expanded container (it was after the metadata block).
- Padding on `<pre>` tightened from `p-[6px]` to `px-[6px] py-[5px]`.
- The metadata footer is a horizontal `flex` with `gap-[14px]` and a dashed `border-t` separator above it (`pt-[5px]` for breathing room).
- Only PATH and SESSION render here, both with `inline` set so they use the new compact MetaRow style.
- TIME / TOOL ID / TTY / PERM rows are **gone**.
- The "更多 / 收起" button is **gone**.

- [ ] **Step 5: TypeScript check**

```bash
make lint
```

Expected: PASS. If TS complains:
- About `formatTimestamp` being undefined — confirm Step 2 removed all references (`grep -n formatTimestamp frontend/src/components/SessionCard.tsx` should return zero hits).
- About removed icon imports — confirm Step 1 only removed icons that no longer have any usage in the file (`grep -n 'ChevronDown\|ChevronUp\|Clock\|ShieldCheck\|Terminal\|Wrench' frontend/src/components/SessionCard.tsx` should return zero hits).

- [ ] **Step 6: Manual visual verification (REQUIRED — no test framework)**

```bash
make dev
```

Trigger a session with content (e.g. via a CC hook firing on this very repo while `make dev` runs, or send a manual event with the bridge if available). Then in the running app, click a card to expand and verify each item below:

| Check | Expected |
|---|---|
| C1. `<pre>` is the first thing visible after the summary row | Pass |
| C2. Footer below `<pre>` shows ONLY `📁 path…` and `# session…`, on a single horizontal row | Pass |
| C3. The footer is separated from `<pre>` by a thin dashed line (top border) | Pass |
| C4. Hovering the footer makes a Copy button fade in next to each value | Pass |
| C5. Clicking Copy on PATH swaps icon to `Check` for ~800ms; clipboard contains the CWD | Pass |
| C6. The "更多 / 收起" button is **gone** | Pass |
| C7. The `🕐 TIME` row is **gone** | Pass |
| C8. There is no way to reveal `🔧 TOOL ID`, `⌨ TTY`, `🛡 PERM` (rows + state are removed) | Pass |
| C9. Card collapsed-state header + summary row look exactly the same as before | Pass |
| C10. Total expanded-card height is visibly shorter than before (≈ 60% of old height) | Pass |

If any check fails, **do not commit** — fix the issue and re-verify before proceeding.

Stop the dev server.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/SessionCard.tsx
git -c commit.gpgsign=false commit -m "feat(ui): compact SessionCard expanded layout

Spec: docs/superpowers/specs/2026-06-02-session-card-compact-layout-design.md

Visible changes (expanded card only — collapsed state untouched):
- <pre> with content_raw is now the first thing rendered (visual focus
  shift; was after the PATH/SESSION/TIME metadata block).
- PATH and SESSION moved to a single-row footer below <pre>, separated
  by a dashed border-top, rendered via MetaRow inline mode (icon +
  value + hover-revealed Copy, no uppercase label text).
- TIME row removed — UpdatedAt is already shown as relative time in
  the header row, redundant.
- TOOL ID / TTY / PERM rows removed — diagnostic info that users
  almost never read; expand/collapse toggle was net noise.
- '更多 / 收起' button + showMore state removed entirely.
- Tightened expanded-section pl from 22px to 18px and <pre> padding
  from 6px to 5/6px.

Result: expanded card height down ~40% with the high-value content
(the actual command/output) becoming the visual anchor.

No backend / IPC / state-machine changes."
```

---

## Task 4: Update regression checklist

**Files:**
- Modify: `docs/regression/2026-06-01-ui-state-regression.md` (§B row 5; §C entire section)

- [ ] **Step 1: Update §B row 5**

In `docs/regression/2026-06-01-ui-state-regression.md`, find the line:

```markdown
| 5 | Click Trash2 | All cards in that project vanish; SQLite `t_sessions.deleted_at` populated |
```

(currently line 30) and replace with:

```markdown
| 5 | Click BrushCleaning (扫帚 icon) | All cards in that project vanish; SQLite `t_sessions.deleted_at` populated |
```

Same line, also update row 4 to mention the new icon:

```markdown
| 4 | Hover project header | Trash2 button fades in on the right |
```

becomes:

```markdown
| 4 | Hover project header | BrushCleaning button fades in on the right |
```

- [ ] **Step 2: Rewrite §C**

Replace the entire `## C. Expanded Card` section (currently lines 32-39, the heading line through the row that says `Click "收起"`) with:

```markdown
## C. Expanded Card

| Step | Action | Expected |
|---|---|---|
| 1 | Click any card | Card expands; `<pre>` with content_raw appears as the first block |
| 2 | Look below `<pre>` | A single-row footer shows PATH (FolderOpen icon + value) and SESSION (Hash icon + value), separated from `<pre>` by a dashed top border |
| 3 | Hover the footer | Copy buttons fade in next to PATH and SESSION values |
| 4 | Click Copy on PATH | Icon swaps to Check for ~800ms; clipboard contains the CWD |
| 5 | Verify removed UI | NO "更多 / 收起" button anywhere; NO TIME row; NO TOOL ID / TTY / PERM rows reachable |
| 6 | Click the card again | Card collapses back to the summary row |
```

- [ ] **Step 3: Verify markdown still renders**

```bash
grep -n "^## " docs/regression/2026-06-01-ui-state-regression.md
```

Expected: section list shows §A through §I in order. If §C heading went missing or duplicated, fix it.

- [ ] **Step 4: Commit**

```bash
git add docs/regression/2026-06-01-ui-state-regression.md
git -c commit.gpgsign=false commit -m "docs(regression): update §B/§C for SessionCard compact layout

§B rows 4-5: Trash2 → BrushCleaning (project clear icon swap)
§C: rewrite Expanded Card checks for the new structure
  - <pre> is now the first block (was after metadata)
  - PATH/SESSION rendered as a single-row footer below <pre>
  - Copy buttons hover-revealed instead of always visible
  - TIME / TOOL ID / TTY / PERM / '更多' button verification: must
    be ABSENT (was: present, expand-toggle behavior tested)"
```

---

## Task 5: Final regression pass + merge

- [ ] **Step 1: Run the full Go test/lint suite**

```bash
make test
make lint
go test -race ./internal/... -timeout=120s
```

Expected: all PASS. Backend should be entirely unaffected by this branch — if any of these fail, we picked up an unrelated regression and should investigate before merging.

- [ ] **Step 2: Build the production app**

```bash
make build
open build/bin/Pager.app
```

This launches the bundled binary so you can verify the UI in production-equivalent mode (Wails-native notifications + bundle path, etc.).

- [ ] **Step 3: Run the manual checklist**

Open `docs/regression/2026-06-01-ui-state-regression.md` and walk through every section §A–§I. The two sections that should look different from the previous run are §B (rows 4-5 mention BrushCleaning) and §C (six rows about the new compact expanded layout).

For each row, click ✅ when verified or ❌ if regressed. The minimum bar to merge is **all rows ✅**, including the unchanged sections.

If any row in §A / §D / §E / §F / §G / §H / §I fails, that means this branch broke something unrelated — stop and investigate before merging.

- [ ] **Step 4: Quit Pager.app**

Close it via menu bar → Pager → Quit, or `pkill -f 'build/bin/Pager.app'`.

- [ ] **Step 5: Merge to main**

```bash
git checkout main
git merge --no-ff feature/sessioncard-compact-layout-2026-06-02 -m "Merge feature/sessioncard-compact-layout-2026-06-02: compact card + broom icon

Two user-driven UI tweaks following the 2026-06-01 redesign ship:

1. SessionCard expanded layout simplified — content_raw <pre> is now
   the first block, PATH/SESSION moved to a horizontal footer with
   hover-revealed Copy. TIME, TOOL ID, TTY, PERM rows and the
   '更多 / 收起' toggle removed entirely. Expanded-card vertical
   space cut by ~40%.

2. Per-project clear icon: Trash2 → BrushCleaning (lucide v1.17.0).
   The action sweeps cards out of the list; broom-style imagery reads
   more accurately than a trash can.

Pure frontend; backend / Tracker / IPC contract / state machine
untouched. Regression checklist (§A–§I) all green on bundled build."
```

- [ ] **Step 6: Optional — delete the feature branch**

After confirming main builds and runs cleanly:

```bash
git branch -d feature/sessioncard-compact-layout-2026-06-02
```

Use `-d` (lowercase) — git will refuse if the branch isn't fully merged, which is the safety net we want.

- [ ] **Step 7: Confirm final state**

```bash
git log --oneline -5
git status --short
```

Expected: top commit is the merge commit; working tree clean (or only `??` files).

---

## Self-Review Notes (writer-side checklist applied)

✅ **Spec coverage:**
- §1 Background (problems with current layout) → addressed by Task 3 (delete) + Task 1 (icon)
- §2 G1–G6 goals → G1/G2/G3/G4 = Task 3; G5 = Task 1; G6 = Task 5 (full Go suite)
- §3.1 改动文件清单 → Tasks 1, 3, 4 cover all three files
- §4.1 DOM structure → Task 3 Step 4 has the exact JSX
- §4.2 间距规范表 → Task 3 Step 4 changes match every row of the table (`pl-[22→18]`, `py-[4]→py-[5]` on `<pre>`, `gap-[14px]` on footer, etc.)
- §4.3 MetaRow inline mode → Task 2 implements with the exact prop name
- §4.4 Icon swap diff → Task 1 Steps 1–2
- §4.5 State machine simplification (delete `showMore`) → Task 3 Step 3
- §6.2 Manual checklist updates → Task 4
- §9 Suggested order → followed (with Task 4-5 collapsed for fewer commits, since they're trivial)

✅ **No placeholders:** Every code step has actual JSX/TS/markdown, every command step has the exact command and expected output. No "implement appropriately" / "add validation" / "similar to Task N".

✅ **Type consistency:**
- `MetaRow` prop signature in Task 2 (`Icon, label, value, copyable, inline`) matches the call sites in Task 3 (`<MetaRow Icon={...} label="PATH" value={...} copyable inline />`).
- `BrushCleaning` is the exact lucide-react export name (verified against `frontend/node_modules/lucide-react/dist/esm/icons/brush-cleaning.mjs`).
- All Tailwind classes used (`group/metarow`, `pl-[18px]`, `border-dashed`, `border-t`, `opacity-0`, `group-hover/metarow:opacity-100`) work with the project's Tailwind setup.

✅ **TDD note:** This codebase has no frontend test framework (verified — there's no `vitest`/`jest` config in `frontend/`). The plan substitutes thorough manual visual verification (Task 3 Step 6 has 10 explicit checks) and the existing manual regression checklist as the validation mechanism. This is consistent with how the prior 2026-06-01 redesign was shipped.
