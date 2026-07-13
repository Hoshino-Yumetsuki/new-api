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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const botProtectionSchema = z.object({
  BotProtectionEnabled: z.boolean(),
  BotProtectionProvider: z.enum(['turnstile', 'capjs']),
  TurnstileSiteKey: z.string().optional(),
  TurnstileSecretKey: z.string().optional(),
  CapJsSecretKey: z.string().optional(),
})

type BotProtectionFormValues = z.infer<typeof botProtectionSchema>

type BotProtectionSectionProps = {
  defaultValues: BotProtectionFormValues
}

export function BotProtectionSection({
  defaultValues,
}: BotProtectionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<BotProtectionFormValues>({
    resolver: zodResolver(botProtectionSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const provider = form.watch('BotProtectionProvider')
  const enabled = form.watch('BotProtectionEnabled')

  const onSubmit = async (data: BotProtectionFormValues) => {
    const legacyTurnstile =
      data.BotProtectionEnabled && data.BotProtectionProvider === 'turnstile'
    const legacyCap =
      data.BotProtectionEnabled && data.BotProtectionProvider === 'capjs'

    const rows: Array<{ key: string; value: string | boolean }> = [
      { key: 'BotProtectionEnabled', value: data.BotProtectionEnabled },
      { key: 'BotProtectionProvider', value: data.BotProtectionProvider },
      { key: 'TurnstileCheckEnabled', value: legacyTurnstile },
      { key: 'CapJsCheckEnabled', value: legacyCap },
      { key: 'CapJsApiEndpoint', value: '' },
      { key: 'TurnstileSiteKey', value: data.TurnstileSiteKey ?? '' },
      { key: 'TurnstileSecretKey', value: data.TurnstileSecretKey ?? '' },
      { key: 'CapJsSecretKey', value: data.CapJsSecretKey ?? '' },
    ]

    for (const { key, value } of rows) {
      const prev = defaultValues[key as keyof BotProtectionFormValues]
      const comparablePrev =
        key === 'TurnstileCheckEnabled'
          ? defaultValues.BotProtectionEnabled &&
            defaultValues.BotProtectionProvider === 'turnstile'
          : key === 'CapJsCheckEnabled'
            ? defaultValues.BotProtectionEnabled &&
              defaultValues.BotProtectionProvider === 'capjs'
            : key === 'CapJsApiEndpoint'
              ? ''
              : prev
      if (value !== comparablePrev) {
        await updateOption.mutateAsync({ key, value })
      }
    }
  }

  return (
    <SettingsSection title={t('Bot Protection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='BotProtectionEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable bot protection')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Require human verification on login, registration, and password recovery'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled ? (
            <FormField
              control={form.control}
              name='BotProtectionProvider'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Bot protection provider')}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='turnstile'>
                        {t('Cloudflare Turnstile')}
                      </SelectItem>
                      <SelectItem value='capjs'>{t('Cap.js (built-in)')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {provider === 'capjs'
                      ? t(
                          'Cap.js runs inside this server. No separate Cap instance is required.'
                        )
                      : t(
                          'Use your Cloudflare Turnstile site key and secret key below.'
                        )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          ) : null}

          {enabled && provider === 'turnstile' ? (
            <>
              <FormField
                control={form.control}
                name='TurnstileSiteKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Site Key')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Your Turnstile site key')}
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='TurnstileSecretKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Secret Key')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t('Your Turnstile secret key')}
                        autoComplete='new-password'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          ) : null}

          {enabled && provider === 'capjs' ? (
            <FormField
              control={form.control}
              name='CapJsSecretKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Cap.js secret key')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t(
                        'Optional. At least 16 characters. Leave empty to use server default.'
                      )}
                      autoComplete='new-password'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Used for Cap.js siteverify. Built-in widget API is always served at /api/cap/builtin/.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          ) : null}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}