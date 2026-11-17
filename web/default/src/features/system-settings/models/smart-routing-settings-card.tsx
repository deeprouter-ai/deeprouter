/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { getSmartRoutingStatus } from './smart-routing-api'

// react-hook-form reads a dotted field name as a path, so the form holds
// { smart_router_setting: { jev_enabled, jev_api_key } } and saves each value
// under its dotted option key.
const ENABLED_KEY = 'smart_router_setting.jev_enabled'
const API_KEY_KEY = 'smart_router_setting.jev_api_key'
const STATUS_QUERY_KEY = ['smart-routing-status']

const smartRoutingSchema = z.object({
  smart_router_setting: z.object({
    jev_enabled: z.boolean(),
    // Always empty on load: the server never returns a saved key. Typing one means "save this".
    jev_api_key: z.string(),
  }),
})

type SmartRoutingFormValues = z.infer<typeof smartRoutingSchema>

interface Props {
  defaultValues: SmartRoutingFormValues
}

/** Admin switch for letting Jev judge each request's task type, plus the TypeSafe key it needs (Smart Routing V2, D4 / D6). */
export function SmartRoutingSettingsCard(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()
  const status = useQuery({
    queryKey: STATUS_QUERY_KEY,
    queryFn: getSmartRoutingStatus,
  })
  const keyConfigured = status.data?.data?.jev_api_key_configured ?? false

  const form = useForm<SmartRoutingFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(smartRoutingSchema) as any,
    defaultValues: props.defaultValues,
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  useResetForm(form as any, props.defaultValues)

  /** Saves the switch when it changed, and the key when one was typed. */
  const onSubmit = async (data: SmartRoutingFormValues) => {
    const enabled = data.smart_router_setting.jev_enabled
    if (enabled !== props.defaultValues.smart_router_setting.jev_enabled) {
      await updateOption.mutateAsync({ key: ENABLED_KEY, value: enabled })
    }
    const apiKey = data.smart_router_setting.jev_api_key.trim()
    if (apiKey) {
      await updateOption.mutateAsync({ key: API_KEY_KEY, value: apiKey })
      form.setValue('smart_router_setting.jev_api_key', '')
      await queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY })
    }
  }

  /** Removes the saved key, so Jev stops until a new one is saved. */
  const clearKey = async () => {
    await updateOption.mutateAsync({ key: API_KEY_KEY, value: '' })
    await queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY })
  }

  return (
    <SettingsSection
      title={t('Smart Routing')}
      description={t('Configure how deeprouter-auto picks a model')}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <FormField
            control={form.control}
            name='smart_router_setting.jev_enabled'
            render={({ field }) => (
              <FormItem className='flex items-start gap-2'>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <div className='space-y-1'>
                  <FormLabel>
                    {t(
                      'Use Jev to judge the request type when picking a model'
                    )}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'When on, deeprouter-auto first asks Jev (by TypeSafe) what kind of task each request is and how hard it is, then picks the model by the routing rules. Adds about 0.4 seconds per request and takes effect within 30 seconds. Kids accounts never use it, and if Jev is slow or down, requests fall back to the keyword rules.'
                    )}
                  </FormDescription>
                  <FormDescription>
                    {t(
                      'An excerpt of each user prompt (the first 300 and last 200 characters) is sent to TypeSafe. Jev needs the TypeSafe key below; without one this switch has no effect.'
                    )}
                  </FormDescription>
                </div>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='smart_router_setting.jev_api_key'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('TypeSafe API key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    autoComplete='off'
                    placeholder={
                      keyConfigured
                        ? t('Enter a new key to replace the saved one')
                        : t('Enter the key')
                    }
                    {...field}
                  />
                </FormControl>
                {status.data ? (
                  <FormDescription>
                    {keyConfigured
                      ? t('A key is saved. It is never shown here.')
                      : t('No key saved yet.')}{' '}
                    {keyConfigured ? (
                      <button
                        type='button'
                        className='underline'
                        onClick={clearKey}
                        disabled={updateOption.isPending}
                      >
                        {t('Clear the saved key')}
                      </button>
                    ) : null}
                  </FormDescription>
                ) : null}
              </FormItem>
            )}
          />

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
