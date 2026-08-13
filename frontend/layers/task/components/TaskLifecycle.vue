<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { FetchError } from 'ofetch'
import type { ApiConfirmation, ApiTask, ImpactPlan } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'

const props = defineProps<{ task: ApiTask }>()
const emit = defineEmits<{ updated: [task: ApiTask] }>()
const api = useTasmaneApi(), router = useRouter()
const confirmations = ref<ApiConfirmation[]>([])
const form = reactive({ content: '', targetName: '', dueAt: '' })
const impact = ref<ImpactPlan>()
const pending = ref<{ kind: 'transition'; status: string } | { kind: 'delete' }>()
const errorMessage = ref('')
onMounted(async () => { try { confirmations.value = await api.confirmations(props.task.id) } catch { errorMessage.value = '確認事項を取得できませんでした。' } })
const transitions = computed(() => props.task.status === 'NOT_STARTED' ? ['IN_PROGRESS'] : props.task.status === 'IN_PROGRESS' ? ['NOT_STARTED', 'COMPLETED'] : ['IN_PROGRESS'])
const label = (status: string) => ({ NOT_STARTED: '未着手へ戻す', IN_PROGRESS: '進行中にする', COMPLETED: '完了する' })[status] || status
const planFrom = (error: unknown) => ((error as FetchError<ImpactPlan>)?.data?.code === 'IMPACT_CONFIRMATION_REQUIRED' ? (error as FetchError<ImpactPlan>).data : undefined)
async function transition(status: string, token?: string) { try { const saved = await api.transitionTask(props.task.id, status, props.task.version, token); impact.value = undefined; pending.value = undefined; emit('updated', saved) } catch (error) { const plan = planFrom(error); if (plan) { impact.value = plan; pending.value = { kind: 'transition', status } } else errorMessage.value = '状態を変更できませんでした。' } }
async function remove(token?: string) { try { await api.deleteTask(props.task.id, props.task.version, token); await router.push('/dashboard') } catch (error) { const plan = planFrom(error); if (plan) { impact.value = plan; pending.value = { kind: 'delete' } } else errorMessage.value = '削除できませんでした。' } }
async function confirmImpact() { if (pending.value?.kind === 'transition') await transition(pending.value.status, impact.value?.impactToken); else if (pending.value?.kind === 'delete') await remove(impact.value?.impactToken) }
async function addConfirmation() { try { confirmations.value.push(await api.createConfirmation(props.task.id, { content: form.content, targetName: form.targetName, dueAt: form.dueAt ? new Date(form.dueAt).toISOString() : null })); form.content = ''; form.targetName = ''; form.dueAt = '' } catch { errorMessage.value = '確認事項を追加できませんでした。' } }
async function setConfirmation(item: ApiConfirmation, status: string) { try { const saved = await api.updateConfirmation(item.id, status); confirmations.value.splice(confirmations.value.findIndex(value => value.id === item.id), 1, saved) } catch { errorMessage.value = '確認状態を変更できませんでした。' } }
const confirmationTransitions = (status: string) => status === 'UNCONFIRMED' ? ['CHECKING'] : status === 'CHECKING' ? ['UNCONFIRMED', 'CONFIRMED'] : ['CHECKING']
</script>
<template><div class="grid two"><section class="card"><h2>状態</h2><p v-if="errorMessage" role="alert">{{ errorMessage }}</p><div class="actions"><button v-for="status in transitions" :key="status" type="button" @click="transition(status)">{{ label(status) }}</button><button class="danger" type="button" @click="remove()">削除</button></div></section><section class="card"><h2>確認事項</h2><ul><li v-for="item in confirmations" :key="item.id"><strong>{{ item.content }}</strong> — {{ item.targetName }} / {{ item.status }}<div class="actions"><button v-for="status in confirmationTransitions(item.status)" :key="status" type="button" @click="setConfirmation(item, status)">{{ status }}</button></div></li></ul><form class="form compact" @submit.prevent="addConfirmation"><label>内容<input v-model="form.content" required /></label><label>確認先<input v-model="form.targetName" required /></label><label>期限<input v-model="form.dueAt" type="datetime-local" /></label><button type="submit">確認事項を追加</button></form></section><ImpactModal :open="Boolean(impact)" title="操作の影響を確認" :impacts="[...(impact?.warnings || []), ...(impact?.affectedEntities.map(item => `${item.title || item.id}: ${item.reason || item.type}`) || [])]" @cancel="impact=undefined; pending=undefined" @confirm="confirmImpact" /></div></template>
