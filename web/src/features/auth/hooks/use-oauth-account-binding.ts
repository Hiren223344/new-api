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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import type { useAccountSecurity } from '@/features/security/hooks/use-account-security'
import { useStatus } from '@/hooks/use-status'
import { api } from '@/lib/api'
import { buildOAuthAuthorizationUrl } from '@/lib/oauth'
import {
  AuthOperationError,
  authRequestOptions,
  authResult,
} from '@/lib/secure-verification'

import { createOAuthAuthorization } from '../api'
import { openOAuthPopup, type OAuthPopupExchange } from '../lib/oauth-popup'
import type { AccountSecurityResult } from '@/features/profile/types'

type PreparedOAuthBinding = AccountSecurityResult & {
  provider: string
  state: string
  url: string
}

type AccountSecurity = ReturnType<typeof useAccountSecurity>

/**
 * Shared "bind provider X to this account" ceremony: security verification,
 * then an OAuth authorization popup, then the callback exchange. Used by the
 * Security page's account bindings list and by the Telegram channel gate,
 * which both need to link a provider from an already-authenticated session.
 * Takes an existing `useAccountSecurity()` instance so a caller that also
 * performs other security-gated actions (e.g. unbinding) shares one pending
 * state and one verification dialog instead of running two independently.
 */
export function useOAuthAccountBinding(
  security: AccountSecurity,
  onBound: () => void
) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [prepared, setPrepared] = useState<PreparedOAuthBinding | null>(null)

  const startBinding = useCallback(
    async (provider: string) => {
      const result = await security.run(async (signal) => {
        const proof = await security.verify(
          { scope: 'account.binding.bind', context: { provider } },
          signal
        )
        const authorization = await createOAuthAuthorization(
          provider,
          'bind',
          undefined,
          signal,
          proof
        )
        return {
          provider,
          state: authorization.state,
          url:
            authorization.authorizationUrl ??
            buildOAuthAuthorizationUrl(provider, authorization.state, status ?? {}),
          notification_warning: false,
        }
      })
      if (result) setPrepared(result)
    },
    [security, status]
  )

  const completeBinding = useCallback(async () => {
    if (!prepared) return
    const target = prepared
    setPrepared(null)
    const result = await security.run(async (signal) => {
      let exchange: OAuthPopupExchange | undefined
      try {
        exchange = await openOAuthPopup({
          provider: target.provider,
          intent: 'bind',
          signal,
          prepare: async () => ({ state: target.state, url: target.url }),
        })
        const callback = exchange.callback
        const outcome = await authResult<AccountSecurityResult>(
          api.get(`/api/oauth/${target.provider}`, {
            ...authRequestOptions,
            singleUseAuthorization: true,
            disableDuplicate: true,
            signal: exchange.signal,
            params: {
              state: callback.state,
              code: callback.code,
              error: callback.error,
              error_description: callback.errorDescription,
            },
          })
        )
        exchange.signal.throwIfAborted()
        exchange.finish({ success: true })
        return outcome
      } catch (error) {
        const failure = AuthOperationError.from(
          exchange?.signal.aborted ? exchange.signal.reason : error
        )
        exchange?.finish({ success: false, message: failure.message })
        throw failure
      }
    })
    if (result) {
      toast.success(t('Binding successful!'))
      onBound()
    }
  }, [prepared, security, onBound, t])

  return {
    prepared,
    startBinding,
    completeBinding,
    cancelPrepared: () => setPrepared(null),
  }
}
