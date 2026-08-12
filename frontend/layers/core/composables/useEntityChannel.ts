import type { Ref } from 'vue'
import { acceptSnapshot, type Versioned } from './taskState'

const entityKinds = ['task', 'genre', 'taskType', 'confirmation'] as const
type EntityKind = typeof entityKinds[number]
export interface EntitySnapshotMessage { type: 'ENTITY_SNAPSHOT'; entity: Versioned & { kind: EntityKind; payload: Record<string, unknown> }; sentAt: string }
export interface EntityInvalidatedMessage { type: 'ENTITY_INVALIDATED'; entityKind: EntityKind; entityId: string; version: number; sentAt: string }
export type EntityMessage = EntitySnapshotMessage | EntityInvalidatedMessage

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value)
const hasExactKeys = (value: Record<string, unknown>, keys: string[]) => {
  const actual = Object.keys(value).sort()
  return actual.length === keys.length && actual.every((key, index) => key === [...keys].sort()[index])
}
const isKind = (value: unknown): value is EntityKind => typeof value === 'string' && entityKinds.includes(value as EntityKind)
const isVersion = (value: unknown): value is number => Number.isInteger(value) && Number(value) >= 1
const rfc3339Pattern = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-](\d{2}):(\d{2}))$/
const isTimestamp = (value: unknown): value is string => {
  if (typeof value !== 'string') return false
  const match = rfc3339Pattern.exec(value)
  if (!match) return false
  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const hour = Number(match[4])
  const minute = Number(match[5])
  const second = Number(match[6])
  const offsetHour = Number(match[7] ?? 0)
  const offsetMinute = Number(match[8] ?? 0)
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate()
  return year >= 1 && month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth && hour <= 23 && minute <= 59 && second <= 59 && offsetHour <= 23 && offsetMinute <= 59 && !Number.isNaN(Date.parse(value))
}

export function parseEntityMessage(data: unknown): EntityMessage | undefined {
  if (!isRecord(data) || typeof data.type !== 'string') return undefined
  if (data.type === 'ENTITY_SNAPSHOT') {
    if (!hasExactKeys(data, ['type', 'entity', 'sentAt']) || !isRecord(data.entity)) return undefined
    const entity = data.entity
    if (!hasExactKeys(entity, ['kind', 'id', 'version', 'payload']) || !isKind(entity.kind) || typeof entity.id !== 'string' || !uuidPattern.test(entity.id) || !isVersion(entity.version) || !isRecord(entity.payload) || !isTimestamp(data.sentAt)) return undefined
    return data as unknown as EntitySnapshotMessage
  }
  if (data.type === 'ENTITY_INVALIDATED') {
    if (!hasExactKeys(data, ['type', 'entityKind', 'entityId', 'version', 'sentAt']) || !isKind(data.entityKind) || typeof data.entityId !== 'string' || !uuidPattern.test(data.entityId) || !isVersion(data.version) || !isTimestamp(data.sentAt)) return undefined
    return data as unknown as EntityInvalidatedMessage
  }
  return undefined
}

export function applyEntityMessage<T extends Versioned>(entityKind: EntityKind, current: T | undefined, message: EntityMessage | undefined): T | undefined {
  if (!message || !current) return current
  if (message.type === 'ENTITY_INVALIDATED') return message.entityKind === entityKind && message.entityId === current.id && message.version >= current.version ? undefined : current
  if (message.entity.kind !== entityKind || message.entity.id !== current.id || !acceptSnapshot(current, message.entity)) return current
  const payload = message.entity.payload
  return payload.id === message.entity.id && payload.version === message.entity.version ? payload as T : current
}

export function useEntityChannel<T extends Versioned>(entityKind: EntityKind, entity: Ref<T | undefined>) {
  if (!import.meta.client || typeof BroadcastChannel === 'undefined') return { publish: () => undefined }
  const channel = new BroadcastChannel('tasmane-entities')
  channel.onmessage = ({ data }: MessageEvent<unknown>) => {
    entity.value = applyEntityMessage(entityKind, entity.value, parseEntityMessage(data))
  }
  onScopeDispose(() => channel.close())
  return { publish: (value: T) => channel.postMessage({ type: 'ENTITY_SNAPSHOT', entity: { kind: entityKind, id: value.id, version: value.version, payload: value }, sentAt: new Date().toISOString() }) }
}
