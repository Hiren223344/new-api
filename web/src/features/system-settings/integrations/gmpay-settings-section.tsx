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
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { SettingsSwitchField } from '../components/settings-form-layout'

export interface GmpaySettingsValues {
  GmpayEnabled: boolean
  GmpayDomain: string
  GmpayPid: string
  GmpaySecret: string
  GmpayCurrency: string
  GmpayUnitPrice: number
  GmpayMinTopUp: number
  GmpayNotifyUrl: string
}

interface Props {
  values: GmpaySettingsValues
  onValueChange: <K extends keyof GmpaySettingsValues>(
    key: K,
    value: GmpaySettingsValues[K]
  ) => void
}

export function GmpaySettingsSection({ values, onValueChange }: Props) {
  const { t } = useTranslation()

  return (
    <div className='space-y-4 pt-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('GM Pay')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t('Crypto top-up gateway, configured the same way as Epay.')}
        </p>
      </div>
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'Obtain the domain, merchant PID, and API secret from your GM Pay account, then configure the notify URL below (or leave it blank to use the automatic callback address).'
          )}
        </AlertDescription>
      </Alert>

      <SettingsSwitchField
        checked={values.GmpayEnabled}
        onCheckedChange={(checked) => onValueChange('GmpayEnabled', checked)}
        label={t('Enable GM Pay')}
        className='py-0'
      />

      <div className='grid gap-1.5'>
        <Label>{t('Domain')}</Label>
        <Input
          placeholder='https://pay.example.com'
          value={values.GmpayDomain}
          onChange={(event) => onValueChange('GmpayDomain', event.target.value)}
        />
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Merchant PID')}</Label>
          <Input
            value={values.GmpayPid}
            onChange={(event) => onValueChange('GmpayPid', event.target.value)}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('API Secret')}</Label>
          <Input
            type='password'
            autoComplete='new-password'
            value={values.GmpaySecret}
            onChange={(event) =>
              onValueChange('GmpaySecret', event.target.value)
            }
          />
        </div>
      </div>

      <div className='grid grid-cols-3 gap-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Currency')}</Label>
          <Input
            placeholder='USD'
            value={values.GmpayCurrency}
            onChange={(event) =>
              onValueChange('GmpayCurrency', event.target.value)
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Unit price (USD)')}</Label>
          <Input
            type='number'
            step={0.1}
            min={0}
            value={values.GmpayUnitPrice}
            onChange={(event) =>
              onValueChange(
                'GmpayUnitPrice',
                event.target.value === '' ? 0 : event.target.valueAsNumber
              )
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Minimum Top-up')}</Label>
          <Input
            type='number'
            min={1}
            value={values.GmpayMinTopUp}
            onChange={(event) =>
              onValueChange(
                'GmpayMinTopUp',
                Number.parseInt(event.target.value, 10) || 0
              )
            }
          />
        </div>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Notify URL (optional)')}</Label>
        <Input
          placeholder='https://your-domain.com/api/gmpay/webhook'
          value={values.GmpayNotifyUrl}
          onChange={(event) =>
            onValueChange('GmpayNotifyUrl', event.target.value)
          }
        />
      </div>
    </div>
  )
}
