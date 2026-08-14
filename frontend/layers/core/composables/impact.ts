import type { FetchError } from 'ofetch'
import type { ImpactPlan } from './useCriticalData'

export function impactPlanFrom(error: unknown): ImpactPlan | undefined {
  const data = (error as FetchError<ImpactPlan>)?.data
  return data?.code === 'IMPACT_CONFIRMATION_REQUIRED' ? data : undefined
}

export function impactLines(plan: ImpactPlan | undefined): string[] {
  if (!plan) return []
  const entities = plan.affectedEntities.map(item => `${item.title || item.id}: ${item.reason || item.type}`)
  return [...plan.warnings, ...entities]
}
