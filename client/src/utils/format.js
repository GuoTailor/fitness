// 展示格式化工具（金额单位为分）

/** 分 → 元字符串（200000 → "2000"，20500 → "205"，20050 → "200.5"） */
export function yuan(fen) {
  if (fen === null || fen === undefined || isNaN(fen)) return '0'
  const v = Number(fen) / 100
  return Number.isInteger(v) ? String(v) : String(Math.round(v * 100) / 100)
}

/** 分 → 带两位小数（20500 → "205.00"） */
export function yuanFixed(fen) {
  return (Number(fen) / 100).toFixed(2)
}

/** 手机号脱敏 13800138000 → 138****8000 */
export function maskPhone(phone) {
  if (!phone || phone.length < 11) return phone || ''
  return phone.replace(/^(\d{3})\d{4}(\d{4})$/, '$1****$2')
}

/** 时间格式化 YYYY-MM-DD HH:mm */
export function fmtTime(input) {
  if (!input) return ''
  const d = new Date(input.replace ? input.replace(/-/g, '/') : input)
  if (isNaN(d.getTime())) return String(input)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/** 日期格式化 YYYY-MM-DD */
export function fmtDate(input) {
  return fmtTime(input).slice(0, 10)
}

/** 星期几（中文） */
export function weekday(input) {
  if (!input) return ''
  const d = new Date(input.replace ? input.replace(/-/g, '/') : input)
  return ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()] || ''
}
