// 生成 Material Icons Round 图标字体子集（npm 离线方案，无需外网）：
// 1. fontkit 打开 round 字体，建 glyphId→codepoint 反查表
// 2. fontkit.layout 对每个图标名做连字 shaping 得到目标字形，反查出其 PUA 码点
// 3. subset-font 按码点文本子集化
// 4. 产出 src/styles/icons.scss（base64 内嵌 @font-face + 每图标 content 码点类）
// 用法：node scripts/build_icon_font.mjs
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import * as fontkit from 'fontkit'
import subsetFont from 'subset-font'

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)))

// 页面用到的图标全集（Material Icons Round 名称）
const ICONS = [
  'home', 'shopping_bag', 'person', 'arrow_back', 'arrow_forward', 'chevron_right',
  'close', 'check', 'check_circle', 'cancel', 'star', 'star_half', 'star_border',
  'favorite', 'share', 'content_copy', 'qr_code', 'qr_code_scanner', 'delete', 'edit',
  'add', 'remove', 'schedule', 'payments', 'confirmation_number', 'sell',
  'shopping_cart', 'receipt_long', 'chat_bubble', 'question_answer', 'forum',
  'reviews', 'thumb_up', 'photo_camera', 'add_a_photo', 'location_on', 'call',
  'verified', 'workspace_premium', 'school', 'groups', 'bar_chart', 'insights',
  'trending_up', 'notifications', 'logout', 'settings', 'refresh', 'search',
  'filter_list', 'expand_more', 'expand_less', 'more_vert', 'block', 'visibility',
  'visibility_off', 'send', 'task_alt', 'history', 'event_available',
  'hourglass_empty', 'monitor_weight', 'login', 'how_to_reg', 'info', 'warning',
  'help', 'support_agent', 'calendar_month', 'event', 'list', 'redeem',
  'local_fire_department', 'emoji_events', 'rocket_launch', 'group_add', 'person_add',
  'assignment', 'description', 'account_balance_wallet', 'inventory_2', 'category',
  'new_releases', 'campaign', 'savings', 'percent', 'lock', 'manage_accounts',
  'badge', 'mail', 'done_all', 'pending', 'pending_actions', 'autorenew', 'error',
  'touch_app', 'fitness_center', 'timer', 'restaurant', 'water_drop', 'straighten',
  'speed', 'today', 'date_range', 'grid_view', 'view_list', 'sort', 'swap_vert',
  'inbox', 'south', 'north'
]

const fontFile = path.join(root, 'node_modules', 'material-icons', 'iconfont', 'material-icons-round.woff2')
if (!fs.existsSync(fontFile)) {
  console.error('未找到 material-icons 包字体，请先 npm install')
  process.exit(1)
}

const fk = fontkit.openSync(fontFile)

// ---- 1. 反查表：glyphId -> codepoint ----
const glyphToCp = new Map()
for (const cp of fk.characterSet) {
  const g = fk.glyphForCodePoint(cp)
  if (g && g.id >= 0 && !glyphToCp.has(g.id)) glyphToCp.set(g.id, cp)
}

// ---- 2. 连字 shaping：图标名 → 字形 → 码点 ----
const codepoints = {}
const failed = []
for (const name of ICONS) {
  const gs = fk.layout(name).glyphs
  if (gs.length !== 1) {
    failed.push(name + `(${gs.length}g)`)
    continue
  }
  const cp = glyphToCp.get(gs[0].id)
  if (!cp) {
    failed.push(name + '(no-cp)')
    continue
  }
  codepoints[name] = cp
}
if (failed.length) {
  console.error('以下图标无法解析，请检查名称或改用其他图标：', failed.join(', '))
  process.exit(1)
}

// ---- 3. 子集化（静态字体，无需变体轴）----
const text = Object.values(codepoints).map((cp) => String.fromCodePoint(cp)).join('')
const subset = await subsetFont(fs.readFileSync(fontFile), text, { targetFormat: 'woff2' })

// ---- 4. 生成 icons.scss ----
// 类名保留下划线原名，与 M3Icon 组件的 'md-icon-' + name 拼接一致
const cls = (n) => '.md-icon-' + n
const hex = (cp) => cp.toString(16).toUpperCase().padStart(4, '0')
const rules = Object.entries(codepoints)
  .map(([n, cp]) => `${cls(n)}::before { content: '\\${hex(cp)}'; }`)
  .join('\n')
const scss = `// 本文件由 scripts/build_icon_font.mjs 生成（Material Icons Round 子集），勿手改
// 实心/空心为不同图标名（如 star / star_border），无 FILL 变量轴
@font-face {
  font-family: 'MSR';
  src: url(data:font/woff2;base64,${subset.toString('base64')}) format('woff2');
  font-display: block;
}
.md-icon {
  font-family: 'MSR';
  font-weight: normal;
  font-style: normal;
  line-height: 1;
  letter-spacing: normal;
  text-transform: none;
  display: inline-block;
  white-space: nowrap;
  word-wrap: normal;
  direction: ltr;
  -webkit-font-smoothing: antialiased;
}
${rules}
`

fs.writeFileSync(path.join(root, 'src', 'styles', 'icons.scss'), scss)
console.log(`icons.scss 生成完成：${Object.keys(codepoints).length} 个图标，woff2 ${(subset.length / 1024).toFixed(1)} KB`)
