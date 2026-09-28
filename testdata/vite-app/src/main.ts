import { groupBy } from 'lodash-es'
import { nanoid } from 'nanoid'

export interface Order {
  id: string
  status: 'open' | 'shipped'
  total: number
}

export function summarize(orders: Order[]): Record<string, number> {
  const groups = groupBy(orders, (o) => o.status)
  const out: Record<string, number> = {}
  for (const [status, items] of Object.entries(groups)) {
    out[status] = items.reduce((sum, o) => sum + o.total, 0)
  }
  return out
}

export function newOrder(): Order {
  return { id: nanoid(), status: 'open', total: 0 }
}

// Lazy chunk: report generator is only loaded on demand
document.getElementById('report-btn')?.addEventListener('click', async () => {
  const { generateReport } = await import('./report')
  const el = document.getElementById('report-out')
  if (el) el.textContent = generateReport()
})
