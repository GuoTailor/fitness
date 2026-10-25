// 手机号脱敏：后台列表展示用（教练端排课联系学员需明文，不在此脱敏）
export function maskPhone(p) {
  if (!p) return ''
  const s = String(p)
  if (s.length < 7) return s
  return s.slice(0, 3) + '****' + s.slice(-4)
}
