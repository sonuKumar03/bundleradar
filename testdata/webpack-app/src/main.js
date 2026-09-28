import { groupBy, sum } from 'lodash'

export function summarize(orders) {
  const groups = groupBy(orders, (o) => o.status)
  const out = {}
  for (const [status, items] of Object.entries(groups)) {
    out[status] = sum(items.map((o) => o.total))
  }
  return out
}

// Lazy chunk: report generator is only loaded on demand
document.getElementById('report-btn')?.addEventListener('click', async () => {
  const { generateReport } = await import('./report')
  const el = document.getElementById('report-out')
  if (el) el.textContent = generateReport()
})
