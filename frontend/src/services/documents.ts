import { api } from '@/api'
import type { DocumentSelection, DocumentType, IsmptDocument } from '@/api'
import { session, loadConfig } from '@/stores/session'
import { getSigner } from './signer'

export type DocumentStep = 'prepare' | 'sign' | 'submit' | 'wait' | 'done'

export interface DocumentProgress {
  type: DocumentType
  step: DocumentStep
  document: IsmptDocument | null
}

const FINAL = new Set(['accepted', 'rejected', 'failed'])
const POLL_INTERVAL_MS = 2000
const POLL_LIMIT_MS = 5 * 60 * 1000

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export function isFinal(doc: IsmptDocument): boolean {
  return FINAL.has(doc.status)
}

export async function waitForDocument(
  id: number,
  onUpdate?: (doc: IsmptDocument) => void,
  shouldStop?: () => boolean,
): Promise<IsmptDocument> {
  const started = Date.now()
  for (;;) {
    const { data } = await api.document(id)
    onUpdate?.(data)
    if (isFinal(data)) return data
    if (Date.now() - started > POLL_LIMIT_MS || shouldStop?.()) return data
    await sleep(POLL_INTERVAL_MS)
  }
}

export async function runDocument(
  type: DocumentType,
  selection: DocumentSelection,
  onProgress: (p: DocumentProgress) => void,
  shouldStop?: () => boolean,
): Promise<IsmptDocument> {
  const config = await loadConfig()
  const signer = getSigner(config?.sign_mode ?? session.config?.sign_mode ?? 'ncalayer')
  onProgress({ type, step: 'prepare', document: null })
  const prepared = (await api.prepareDocument(type, selection)).data
  onProgress({ type, step: 'sign', document: prepared })
  const signature = await signer.signCms(prepared.body_base64, { attached: false })
  onProgress({ type, step: 'submit', document: prepared })
  const submitted = (await api.submitDocument(prepared.id, signature)).data
  onProgress({ type, step: 'wait', document: submitted })
  const final = isFinal(submitted)
    ? submitted
    : await waitForDocument(submitted.id, (doc) => onProgress({ type, step: 'wait', document: doc }), shouldStop)
  onProgress({ type, step: 'done', document: final })
  return final
}
