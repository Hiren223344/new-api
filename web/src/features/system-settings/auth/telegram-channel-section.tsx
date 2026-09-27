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

import { Alert, AlertDescription } from '@/components/ui/alert'
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
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const telegramChannelSchema = z.object({
  telegram_channel: z.object({
    enabled: z.boolean(),
    chat_id: z.string(),
    join_link: z.string(),
  }),
  TelegramBotToken: z.string(),
})

type TelegramChannelFormValues = z.infer<typeof telegramChannelSchema>

export type FlatTelegramChannelDefaults = {
  'telegram_channel.enabled': boolean
  'telegram_channel.chat_id': string
  'telegram_channel.join_link': string
  TelegramBotToken: string
}

function buildFormDefaults(
  defaults: FlatTelegramChannelDefaults
): TelegramChannelFormValues {
  return {
    telegram_channel: {
      enabled: defaults['telegram_channel.enabled'],
      chat_id: defaults['telegram_channel.chat_id'],
      join_link: defaults['telegram_channel.join_link'],
    },
    TelegramBotToken: defaults.TelegramBotToken,
  }
}

function normalizeFormValues(
  values: TelegramChannelFormValues
): FlatTelegramChannelDefaults {
  return {
    'telegram_channel.enabled': values.telegram_channel.enabled,
    'telegram_channel.chat_id': values.telegram_channel.chat_id,
    'telegram_channel.join_link': values.telegram_channel.join_link,
    TelegramBotToken: values.TelegramBotToken,
  }
}

type TelegramChannelSectionProps = {
  defaultValues: FlatTelegramChannelDefaults
}

export function TelegramChannelSection({
  defaultValues,
}: TelegramChannelSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<TelegramChannelFormValues>({
    resolver: zodResolver(telegramChannelSchema),
    defaultValues: buildFormDefaults(defaultValues),
  })

  useEffect(() => {
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: TelegramChannelFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.entries(normalized) as [
        keyof FlatTelegramChannelDefaults,
        boolean | string,
      ][]
    ).filter(([key, value]) => value !== defaultValues[key])

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: String(value) })
    }
  }

  return (
    <SettingsSection title={t('Telegram Channel Verification')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <Alert>
            <AlertDescription>
              {t(
                'Add your bot as an administrator of the target channel, so it can confirm whether a user has joined. New and existing users are required to verify before they can use the dashboard or the API.'
              )}
            </AlertDescription>
          </Alert>

          <FormField
            control={form.control}
            name='telegram_channel.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Require Telegram channel membership')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Block sign-in and API access until the user verifies they joined the channel below.'
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

          <FormField
            control={form.control}
            name='TelegramBotToken'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Bot Token')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Bot token from BotFather')}
                    autoComplete='new-password'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Also used for the legacy Telegram bot name shown to users; the bot must be an administrator of the channel below.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='telegram_channel.chat_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Channel ID')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='@your_channel or -1001234567890'
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The channel username or numeric chat ID passed to the Telegram Bot API.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='telegram_channel.join_link'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Join Link')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='https://t.me/your_channel'
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Shown to users so they can actually join the channel.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
