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
import { Send } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { useOAuthAccountBinding } from '@/features/auth/hooks/use-oauth-account-binding'
import { SecureVerificationDialog } from '@/features/auth/secure-verification'
import { useProfile } from '@/features/profile/hooks/use-profile'
import { useAccountSecurity } from '@/features/security/hooks/use-account-security'
import { useStatus } from '@/hooks/use-status'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'

const ADMIN_ROLE_THRESHOLD = 10

interface VerifyChannelResponse {
  success: boolean
  message?: string
  data?: { verified: boolean }
}

/**
 * Blocks dashboard access for a logged-in user who has not yet verified
 * membership in the administrator-configured Telegram channel. Server-side
 * enforcement lives in middleware.RequireTelegramChannelJoin /
 * TokenAuth — this is UX only, so it fails open (renders children) while the
 * profile is still loading rather than guessing at access.
 */
export function TelegramChannelGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { profile, loading: profileLoading, refreshProfile } = useProfile()
  const security = useAccountSecurity()
  const oauthBinding = useOAuthAccountBinding(security, () => {
    void refreshProfile()
  })
  const [verifying, setVerifying] = useState(false)
  const [verifyError, setVerifyError] = useState<string | null>(null)

  const gateEnabled = Boolean(status?.telegram_channel_gate_enabled)
  const isExemptRole = (profile?.role ?? 0) >= ADMIN_ROLE_THRESHOLD
  const blocked =
    gateEnabled &&
    !profileLoading &&
    profile != null &&
    !isExemptRole &&
    !profile.telegram_channel_verified

  if (!blocked) return children

  const handleVerify = async () => {
    setVerifying(true)
    setVerifyError(null)
    try {
      const response = await api.post<VerifyChannelResponse>(
        '/api/user/telegram-channel/verify'
      )
      if (response.data.success && response.data.data?.verified) {
        toast.success(t('Verified! Welcome aboard.'))
        await refreshProfile()
      } else {
        setVerifyError(
          response.data.message ||
            t(
              "We couldn't confirm you've joined yet. Join the channel, then try again."
            )
        )
      }
    } catch (error) {
      handleServerError(error, t('Verification failed. Please try again.'))
    } finally {
      setVerifying(false)
    }
  }

  return (
    <div className='bg-background fixed inset-0 z-50 flex items-center justify-center p-4'>
      <div className='w-full max-w-md space-y-6 rounded-lg border p-6 shadow-lg'>
        <div className='flex flex-col items-center gap-3 text-center'>
          <div className='flex h-12 w-12 items-center justify-center rounded-xl bg-blue-100 dark:bg-blue-900'>
            <Send className='h-6 w-6 text-blue-600 dark:text-blue-400' />
          </div>
          <h1 className='text-lg font-semibold'>{t('One more step')}</h1>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Join our Telegram channel to unlock your dashboard and API access.'
            )}
          </p>
        </div>

        {!profile?.telegram_id ? (
          <div className='space-y-3'>
            <p className='text-center text-sm'>
              {t('First, link your Telegram account.')}
            </p>
            <Button
              className='w-full'
              disabled={security.pending}
              onClick={() => void oauthBinding.startBinding('telegram')}
            >
              {t('Link Telegram account')}
            </Button>
          </div>
        ) : (
          <div className='space-y-3'>
            <Button
              className='w-full'
              variant='outline'
              onClick={() =>
                window.open(
                  status?.telegram_channel_join_link || 'https://telegram.org',
                  '_blank',
                  'noreferrer'
                )
              }
            >
              {t('Join the Telegram channel')}
            </Button>
            <Button
              className='w-full'
              disabled={verifying}
              onClick={() => void handleVerify()}
            >
              {verifying ? t('Checking…') : t("I've joined — verify now")}
            </Button>
            {verifyError && (
              <p className='text-destructive text-center text-sm'>
                {verifyError}
              </p>
            )}
          </div>
        )}
      </div>

      {security.showVerification && (
        <SecureVerificationDialog {...security.verificationDialogProps} />
      )}
      <ConfirmDialog
        open={oauthBinding.prepared !== null}
        onOpenChange={(open) => {
          if (!open) oauthBinding.cancelPrepared()
        }}
        title={t('Continue account binding')}
        desc={t(
          'Your identity has been verified. Continue to the provider to finish linking your account.'
        )}
        handleConfirm={() => void oauthBinding.completeBinding()}
        confirmText={t('Continue')}
      />
    </div>
  )
}
