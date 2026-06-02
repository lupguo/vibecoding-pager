# SessionCard 紧凑展开布局 + 项目清理图标更换 设计文档

**Date:** 2026-06-02
**Scope:** 仅前端（`frontend/src/components/SessionCard.tsx` + `SessionList.tsx`），零后端改动
**Issue source:** 用户反馈卡片展开过于冗长 + 项目清理 icon 语义不直观

---

## 1. 背景

当前 SessionCard 展开态依次渲染：

1. 元信息块 A（PATH / SESSION / TIME，三行带 FolderOpen / Hash / Clock 图标 + Copy 按钮）
2. `content_raw` 代码块（pre）
3. 「更多」折叠区（点击展开 TOOL ID / TTY / PERM 三行带 Wrench / Terminal / ShieldCheck 图标）
4. 「更多 / 收起」切换按钮（带虚线边框 + Chevron）

问题：
- **垂直空间消耗大**：单卡片展开后纵向占用 ≈ 240px，4 个 session 就把弹窗挤满
- **TIME / TOOL ID / TTY / PERM 信息很少被使用**：用户日常关注的是「这条事件是干什么的（content_raw）」+「在哪个项目（PATH）」
- **「更多 / 收起」按钮的存在价值低**：折叠区里的内容用户基本不看，按钮反而成了噪声

同时，项目分组 hover 时显示的「一键清理」按钮当前用 `Trash2`（垃圾桶）。语义上这是「把这个项目下的会话卡片扫掉」，更接近"扫地"而非"丢垃圾"。用户希望换成扫帚类图标。

## 2. 目标

| # | 目标 | 验收 |
|---|---|---|
| G1 | 展开态信息收敛到「概要 + 详情 + 附加项」三层 | 删掉 TIME / TOOL ID / TTY / PERM / 「更多」按钮 |
| G2 | 视觉焦点从"元信息块"转移到`content_raw`代码块 | 代码块在展开容器最顶部 |
| G3 | PATH / SESSION 弱化为脚注 | 单行 flex 小字，与代码块用 dashed border-top 分隔 |
| G4 | 卡片展开高度显著减小 | 展开后纵向占用 ≤ 当前 60% |
| G5 | 项目清理按钮使用扫帚类语义图标 | `Trash2` → `BrushCleaning` |
| G6 | 后端 + 状态机 + IPC 协议零改动 | `make test` / `-race` / 集成测试 0 失败 |

## 3. 范围

### 3.1 改动文件

```
frontend/src/components/SessionCard.tsx     # 删除 4 元信息行 + 「更多」按钮 + showMore state；重排展开态
frontend/src/components/SessionList.tsx     # Trash2 → BrushCleaning
docs/regression/2026-06-01-ui-state-regression.md  # §B.5 / §C 更新
```

### 3.2 不在范围内

- 折叠态（header 行 + summary 行）保持原样
- `StatusIcon` / 4-state 视觉 token（color + icon + animation）保留
- pulse-highlight / 跳转箭头按钮 / Copy → Check 反馈交互保留
- `MetaRow` 组件保留（PATH / SESSION footer 仍在用，只是少了 3 个调用点）
- 后端 `Session` struct、`PendingTools`、`tracker.go`、`notify` 全部不动
- 测试基础设施：仍无前端单元测试框架，沿用手工验收清单

## 4. 详细设计

### 4.1 展开态 DOM 结构（方案 B）

```
<div className="card">
  <div className="header-row">{/* 不变：StatusIcon + WORKING + CC-INT + sessionPrefix + relativeTime */}</div>
  <div className="summary-row">{/* 不变：[Bash] + content + ArrowRight 按钮 */}</div>
  {expanded && (
    <div className="expanded">                                  {/* pl-[18px] (was 22px) */}
      <pre className="content-raw">{/* 视觉焦点，移到最前 */}</pre>
      <div className="footer-meta">                              {/* 弱化为脚注 */}
        <MetaRow Icon={FolderOpen} label="PATH"    value={CWD}       copyable inline />
        <MetaRow Icon={Hash}       label="SESSION" value={SessionID} copyable inline />
      </div>
    </div>
  )}
</div>
```

**关键差异（vs 当前实现）：**

1. 元信息块从展开容器**顶部**移到**底部**
2. PATH 与 SESSION 从**两行 stack** 改为**单行 flex**（在 footer 容器内 horizontal）
3. 删除 `Clock`/TIME 行、`Wrench`/TOOL ID 行、`Terminal`/TTY 行、`ShieldCheck`/PERM 行
4. 删除 `showMore: boolean` state、删除「更多 / 收起」按钮 + `ChevronUp`/`ChevronDown` import
5. `formatTimestamp()` 函数变为死代码 → 删除

### 4.2 间距规范（紧凑度量化）

| 位置 | 当前值 | 新值 |
|---|---|---|
| 展开容器 `padding-left` | `pl-[22px]` | `pl-[18px]` |
| 展开容器 `margin-top` | `mt-[6px]` | `mt-[6px]`（不变） |
| 代码块外层 padding | `p-[6px]` | `p-[5px_6px]` |
| 代码块 `font-size` | `text-[10px]` | `text-[10px]`（不变） |
| 代码块 `max-height` | `max-h-32`（128px） | `max-h-32`（不变） |
| Footer 容器内 padding | — | `pt-[5px]`（dashed border-top 之后） |
| Footer 内行间距 | — | `gap-[14px]` （horizontal flex） |
| Footer `font-size` | （此前 PATH 行 `text-[10px]`） | `text-[10px]` |
| Footer 颜色 | （此前 `#86868b`） | `#86868b` |
| Copy 按钮可见性 | 永久 | **hover 父容器时淡入** |

### 4.3 `MetaRow` 组件 inline 模式（最小变更）

`MetaRow` 当前是 stack 布局（label + value 各占一格）。Footer 中要 horizontal 排列，新增 `inline?: boolean` prop：

```tsx
function MetaRow({ Icon, label, value, copyable, inline }: {
  Icon: typeof FolderOpen
  label: string
  value: string
  copyable?: boolean
  inline?: boolean   // 新增：true 时用 horizontal flex，标签紧贴值
}) {
  // inline=true: <span>📁 [value] [Copy]</span> — 标签其实可省略，靠 icon 表意
  // inline=false: 当前实现
}
```

实施时 `inline=true` 走简化路径（仅 Icon + value + 可选 Copy；不渲染 "PATH" 文字标签，因为图标已经表达），节省横向空间。

### 4.4 图标替换

```diff
- import { ChevronDown, ChevronRight, ChevronUp, Folder, Trash2 } from 'lucide-react'
+ import { BrushCleaning, ChevronDown, ChevronRight, Folder } from 'lucide-react'
```

`SessionList.tsx:73`：

```diff
- <Trash2 size={11} strokeWidth={2} />
+ <BrushCleaning size={11} strokeWidth={2} />
```

`SessionCard.tsx` import：删除 `ChevronUp`（仅「收起」按钮在用）。

### 4.5 状态机简化

`SessionCard.tsx` 当前 component state：

```ts
const [expanded, setExpanded] = useState(false)
const [showMore, setShowMore] = useState(false)        // ← 删除
const [copyState, setCopyState] = useState<...>(...)   // ← 保留
```

删除 `showMore` 后，`onClick` handler 只剩 `setExpanded(!expanded)`，更简单。

## 5. 视觉对比

### 5.1 当前实现（截图所示）

```
┌─────────────────────────────────────┐
│ ◯ WORKING  CC-INT      9327c3f8 11s │  ← header
│ [Bash] CFG=~/.config/pager...    →  │  ← summary
│   ╭─ Meta Block A ──────────────╮   │  ← TIME 行也在这块
│   │ 📁 PATH    /private/...   ⎘ │   │
│   │ #  SESSION 9327c3f8...   ⎘ │   │
│   │ 🕐 TIME    2026/06/02 14:29 │   │
│   ╰────────────────────────────╯   │
│   ┌─ <pre> code ────────────────┐  │
│   │ CFG=~/.config/pager/...     │  │
│   │ cp "$CFG" "$CFG.bak..."     │  │
│   │ ...                         │  │
│   └─────────────────────────────┘  │
│   ╭─ Meta Block B ──────────────╮   │  ← 「更多」点开
│   │ 🔧 TOOL ID toolset_...   ⎘  │   │
│   │ ⌨  TTY     —                │   │
│   │ 🛡 PERM    bypassPermiss... │   │
│   ╰────────────────────────────╯   │
│       ⌃ 收起                        │  ← 切换按钮
└─────────────────────────────────────┘
```

### 5.2 改后实现（方案 B）

```
┌─────────────────────────────────────┐
│ ◯ WORKING  CC-INT      9327c3f8 11s │  ← header（不变）
│ [Bash] CFG=~/.config/pager...    →  │  ← summary（不变）
│   ┌─ <pre> code ────────────────┐  │
│   │ CFG=~/.config/pager/...     │  │  ← 视觉焦点
│   │ cp "$CFG" "$CFG.bak..."     │  │
│   │ ...                         │  │
│   └─────────────────────────────┘  │
│   ┄ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─    │  ← dashed border-top
│   📁 /private/data/...    # 9327c3f8│  ← footer 小字
└─────────────────────────────────────┘
```

预估高度对比：当前 ~240px → 改后 ~140px（减少约 42%）。

## 6. 测试 / 回归策略

### 6.1 自动化（Go 后端）

零改动 → 既有 `make test` / `make lint` / `go test -race ./...` / `go test -tags=integration` 全套不动。

### 6.2 手工验收清单更新

更新 `docs/regression/2026-06-01-ui-state-regression.md`：

**§B「Project Grouping」第 5 步：**

```diff
- | 5 | Click Trash2 | All cards in that project vanish; SQLite t_sessions.deleted_at populated |
+ | 5 | Click BrushCleaning (扫帚) | All cards in that project vanish; SQLite t_sessions.deleted_at populated |
```

**§C「Expanded Card」整段重写：**

```markdown
## C. Expanded Card

| Step | Action | Expected |
|---|---|---|
| 1 | Click any card | Card expands; <pre> with content_raw appears at top |
| 2 | Scroll to the bottom of expanded section | Footer shows PATH and SESSION on a single horizontal line, separated from <pre> by a dashed border-top |
| 3 | Hover footer | Copy buttons fade in next to PATH and SESSION values |
| 4 | Click Copy on PATH | Icon swaps to Check for ~800ms; clipboard contains the CWD |
| 5 | Verify removed UI | NO "更多 / 收起" button, NO TIME row, NO TOOL ID / TTY / PERM rows |
```

### 6.3 手工验收时机

实施时每改完一个 task 都跑一次 `make dev` 视觉确认。最终 commit 前完整跑一遍清单 §A–§I（其他 sections 应该全部维持原状）。

## 7. 风险与回退

| 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|
| 用户后悔删了 TIME 行 | 中 | 低 | 数据仍在 `session.UpdatedAt` 字段；如需恢复改 5 行代码 |
| 用户后悔删了 TOOL ID / TTY / PERM | 低 | 低 | 同上，数据在 `session.LastEvent.tool_use_id` / `session.TTY` / `permission_mode` |
| `BrushCleaning` 图标在小尺寸（11px）下识别度差 | 中 | 低 | 保留 hover tooltip "Clear all sessions in this project" |
| Footer 单行在 PATH 很长时被挤压 | 中 | 中 | `text-overflow: ellipsis + min-w-0`；hover 时 tooltip 显示完整 PATH |
| 删除 `showMore` 引入 React state diff | 低 | 低 | TypeScript 编译会立即抓出未使用的 state setter |

回退路径：`git revert <commit>` 即可，因为本次改动严格隔离在 2 个文件 + 1 个文档内。

## 8. 不做的事 (YAGNI)

- 不引入 framer-motion 等动画库做高度过渡（CSS `max-height` transition 足够）
- 不为 Footer 设计可配置项（永久 horizontal flex）
- 不做"双击 Footer 切回旧布局"的兼容方案（增加复杂度，没价值）
- 不更新 i18n 字符串（"PATH" / "SESSION" 是常量缩写，无需翻译）
- 不在 SettingsPanel 加「展开布局」开关（YAGNI；如未来真有需要再说）

## 9. 实施顺序提示（给 writing-plans 用）

建议任务粒度：

1. **图标替换**（最小风险）：`SessionList.tsx` Trash2 → BrushCleaning + 手工 hover 验证
2. **MetaRow inline 模式**：扩展 `MetaRow` 加 `inline?: boolean` prop，仅修改组件本身
3. **SessionCard 删除冗余结构**：删除 `showMore` state + 「更多」按钮 + 4 行元信息（TIME/TOOL ID/TTY/PERM）+ `formatTimestamp` + 相关 import
4. **重排展开态**：把 PATH/SESSION 移到 `<pre>` 之后，套用 `inline=true`
5. **间距/视觉打磨**：`pl-[22px]` → `pl-[18px]`、Copy 按钮 hover-revealed、dashed border-top
6. **回归文档更新**：`docs/regression/2026-06-01-ui-state-regression.md` §B.5 + §C 重写
7. **手工跑全套清单**

每步独立 commit。完成后无需 PR review，直接合到 main（与之前 UI 改动保持一致的 workflow）。
