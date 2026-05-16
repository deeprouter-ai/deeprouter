/*
"Quick Import Models" dialog for the Models page (mirrors the Channels
Quick Import I shipped earlier). Seeds the model metadata catalog with
a curated DeepRouter-recommended starter pack — 25-ish popular models
grouped into Chat / Reasoning / Image / Video / Audio / Embedding.

Each imported model is created with status=disabled so the operator
reviews before exposing in /v1/models. Models that already exist
(same model_name) are skipped rather than failing the whole batch.

For the actual routing wiring, see Channels — a model only becomes
invokable once a matching Channel includes it in its models field.
This dialog only creates the metadata row.
*/
import { useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { createModel } from '../../api'
import {
  GROUP_LABELS,
  GROUP_ORDER,
  MODEL_PRESETS,
  type ModelPreset,
} from '../../lib/model-presets'

export interface QuickImportModelsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function QuickImportModelsDialog({
  open,
  onOpenChange,
}: QuickImportModelsDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [submitting, setSubmitting] = useState(false)

  // Group presets by category for the rendered checkboxes. Memoized so we
  // don't re-bucket on every render.
  const presetsByGroup = useMemo(() => {
    const map = new Map<string, ModelPreset[]>()
    for (const p of MODEL_PRESETS) {
      const list = map.get(p.group) ?? []
      list.push(p)
      map.set(p.group, list)
    }
    return map
  }, [])

  const selectedCount = useMemo(
    () => Object.values(selected).filter(Boolean).length,
    [selected]
  )

  const toggle = (name: string) =>
    setSelected((s) => ({ ...s, [name]: !s[name] }))

  const toggleAll = (next: boolean) =>
    setSelected(
      Object.fromEntries(MODEL_PRESETS.map((p) => [p.model_name, next]))
    )

  const toggleGroup = (group: string, next: boolean) => {
    const groupNames = (presetsByGroup.get(group) ?? []).map(
      (p) => p.model_name
    )
    setSelected((s) => {
      const out = { ...s }
      for (const n of groupNames) out[n] = next
      return out
    })
  }

  const reset = () => {
    setSelected({})
    setSubmitting(false)
  }

  const createOne = async (preset: ModelPreset) => {
    const result = await createModel({
      model_name: preset.model_name,
      description: preset.description,
      tags: preset.tags.join(','),
      endpoints: preset.endpoints,
      status: 0, // disabled until operator reviews
      sync_official: 1,
      name_rule: 0,
    })
    if (!result?.success) {
      // Some backends return success:false with "model already exists" —
      // treat that as a skip rather than a failure so partial batches
      // don't trip the operator up.
      const msg = (result?.message || '').toLowerCase()
      if (msg.includes('exist')) return { skipped: true as const }
      throw new Error(result?.message || `Failed to create ${preset.model_name}`)
    }
    return { skipped: false as const }
  }

  const handleSubmit = async () => {
    const picks = MODEL_PRESETS.filter((p) => selected[p.model_name])
    if (picks.length === 0) {
      toast.error(t('Pick at least one model'))
      return
    }
    setSubmitting(true)
    let created = 0
    let skipped = 0
    const failures: string[] = []
    for (const p of picks) {
      try {
        const r = await createOne(p)
        if (r.skipped) skipped += 1
        else created += 1
      } catch (e) {
        failures.push(`${p.model_name}: ${(e as Error).message}`)
      }
    }
    setSubmitting(false)
    if (failures.length === 0) {
      const msg =
        skipped > 0
          ? t(
              'Imported {{c}} model(s), skipped {{s}} duplicate(s). Review and enable in the Models table.',
              { c: created, s: skipped }
            )
          : t(
              'Imported {{c}} model(s) as disabled. Review and enable in the Models table.',
              { c: created }
            )
      toast.success(msg)
      queryClient.invalidateQueries({ queryKey: ['models'] })
      onOpenChange(false)
      reset()
    } else {
      toast.error(
        t('Imported {{ok}}/{{total}}. Failures:\n{{msgs}}', {
          ok: created,
          total: picks.length,
          msgs: failures.join('\n'),
        })
      )
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!submitting) {
          if (!v) reset()
          onOpenChange(v)
        }
      }}
    >
      <DialogContent className='max-h-[90vh] overflow-hidden sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Quick Import Models')}</DialogTitle>
          <DialogDescription>
            {t(
              'Seed the model metadata catalog with a curated starter pack. Imported entries are disabled — review then enable per model. A model is only invokable once a Channel includes it in its models field.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='flex items-center justify-between border-b pb-2'>
          <span className='text-muted-foreground text-sm'>
            {t('{{n}} of {{total}} selected', {
              n: selectedCount,
              total: MODEL_PRESETS.length,
            })}
          </span>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => toggleAll(true)}
              disabled={submitting}
            >
              {t('Select all')}
            </Button>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => toggleAll(false)}
              disabled={submitting}
            >
              {t('Clear')}
            </Button>
          </div>
        </div>

        <div className='max-h-[58vh] space-y-5 overflow-y-auto pr-1'>
          {GROUP_ORDER.map((group) => {
            const items = presetsByGroup.get(group) ?? []
            if (items.length === 0) return null
            const allOn = items.every((p) => selected[p.model_name])
            return (
              <div key={group}>
                <div className='mb-2 flex items-center justify-between'>
                  <span className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
                    {GROUP_LABELS[group]}
                  </span>
                  <Button
                    type='button'
                    variant='ghost'
                    size='xs'
                    onClick={() => toggleGroup(group, !allOn)}
                    disabled={submitting}
                  >
                    {allOn ? t('Unselect group') : t('Select group')}
                  </Button>
                </div>
                <div className='space-y-1.5'>
                  {items.map((p) => {
                    const checked = !!selected[p.model_name]
                    return (
                      <Label
                        key={p.model_name}
                        htmlFor={`qim-${p.model_name}`}
                        className='hover:bg-accent/40 flex cursor-pointer items-start gap-3 rounded-md border p-2.5'
                      >
                        <Checkbox
                          id={`qim-${p.model_name}`}
                          checked={checked}
                          onCheckedChange={() => toggle(p.model_name)}
                          disabled={submitting}
                          className='mt-0.5'
                        />
                        <div className='flex-1 space-y-1'>
                          <div className='flex flex-wrap items-center gap-1.5'>
                            <span className='font-mono text-[13px] font-medium'>
                              {p.model_name}
                            </span>
                            {p.tags.map((tag) => (
                              <span
                                key={tag}
                                className='text-muted-foreground bg-muted/60 rounded px-1 py-0.5 text-[10px] tracking-wide uppercase'
                              >
                                {tag}
                              </span>
                            ))}
                          </div>
                          <p className='text-muted-foreground text-xs'>
                            {p.description}
                          </p>
                        </div>
                      </Label>
                    )
                  })}
                </div>
              </div>
            )
          })}
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={handleSubmit}
            disabled={submitting || selectedCount === 0}
          >
            {submitting && <Loader2 className='mr-1 size-4 animate-spin' />}
            {t('Import {{n}} model(s)', { n: selectedCount })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
