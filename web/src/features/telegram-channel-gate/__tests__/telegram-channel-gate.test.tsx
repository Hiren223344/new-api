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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { UserProfile } from '@/features/profile/types'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { TelegramChannelGate } from '../telegram-channel-gate'

const baseProfile: UserProfile = {
  id: 1,
  username: 'alice',
  display_name: 'Alice',
  role: 1,
  group: 'default',
  quota: 1000000,
  used_quota: 0,
  request_count: 0,
  status: 1,
  aff_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  created_time: 0,
}

let currentProfile: UserProfile = baseProfile

beforeEach(() => {
  currentProfile = baseProfile
  useAuthStore.getState().auth.setUser({ ...baseProfile })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/user/self') {
      return { data: { success: true, data: currentProfile } }
    }
    throw new Error(`Unexpected GET ${url}`)
  })
})

afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

function renderGate(status: Record<string, unknown>) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['status'], status)
  return render(
    <QueryClientProvider client={client}>
      <TelegramChannelGate>
        <div>dashboard content</div>
      </TelegramChannelGate>
    </QueryClientProvider>
  )
}

describe('TelegramChannelGate', () => {
  it('renders the dashboard when the gate is disabled', async () => {
    renderGate({ telegram_channel_gate_enabled: false })

    expect(await screen.findByText('dashboard content')).toBeInTheDocument()
  })

  it('renders the dashboard for an exempt admin role even when unverified', async () => {
    currentProfile = { ...baseProfile, role: 10, telegram_channel_verified: false }
    renderGate({ telegram_channel_gate_enabled: true })

    expect(await screen.findByText('dashboard content')).toBeInTheDocument()
  })

  it('blocks and offers to link Telegram when no account is linked yet', async () => {
    currentProfile = { ...baseProfile, telegram_channel_verified: false }
    renderGate({
      telegram_channel_gate_enabled: true,
      telegram_oauth_configured: true,
    })

    const linkButton = await screen.findByRole('button', {
      name: 'Link Telegram account',
    })
    expect(linkButton).toBeInTheDocument()
    expect(linkButton).toBeEnabled()
    expect(screen.queryByText('dashboard content')).not.toBeInTheDocument()
  })

  it('warns and disables linking when Telegram OAuth is not configured', async () => {
    currentProfile = { ...baseProfile, telegram_channel_verified: false }
    renderGate({
      telegram_channel_gate_enabled: true,
      telegram_oauth_configured: false,
    })

    expect(
      await screen.findByText(
        'Telegram OAuth is not configured or enabled. Please contact your administrator.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Link Telegram account' })
    ).toBeDisabled()
    expect(screen.queryByText('dashboard content')).not.toBeInTheDocument()
  })

  it('blocks and offers to verify once Telegram is linked', async () => {
    currentProfile = {
      ...baseProfile,
      telegram_id: '123456',
      telegram_channel_verified: false,
    }
    renderGate({
      telegram_channel_gate_enabled: true,
      telegram_channel_join_link: 'https://t.me/testchannel',
    })

    expect(
      await screen.findByRole('button', { name: 'Join the Telegram channel' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: "I've joined — verify now" })
    ).toBeInTheDocument()
    expect(screen.queryByText('dashboard content')).not.toBeInTheDocument()
  })

  it('unblocks the dashboard after a successful verification', async () => {
    currentProfile = {
      ...baseProfile,
      telegram_id: '123456',
      telegram_channel_verified: false,
    }
    const user = userEvent.setup()
    const postSpy = vi.spyOn(api, 'post').mockImplementation(async (url) => {
      if (url === '/api/user/telegram-channel/verify') {
        currentProfile = { ...currentProfile, telegram_channel_verified: true }
        return { data: { success: true, data: { verified: true } } }
      }
      throw new Error(`Unexpected POST ${url}`)
    })

    renderGate({ telegram_channel_gate_enabled: true })

    await user.click(
      await screen.findByRole('button', { name: "I've joined — verify now" })
    )

    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith('/api/user/telegram-channel/verify')
    })
    expect(await screen.findByText('dashboard content')).toBeInTheDocument()
  })

  it('shows an error and stays blocked when membership cannot be confirmed', async () => {
    currentProfile = {
      ...baseProfile,
      telegram_id: '123456',
      telegram_channel_verified: false,
    }
    const user = userEvent.setup()
    vi.spyOn(api, 'post').mockImplementation(async (url) => {
      if (url === '/api/user/telegram-channel/verify') {
        return {
          data: {
            success: false,
            code: 'TELEGRAM_CHANNEL_NOT_JOINED',
            message: "You haven't joined the required Telegram channel yet.",
          },
        }
      }
      throw new Error(`Unexpected POST ${url}`)
    })

    renderGate({ telegram_channel_gate_enabled: true })
    await user.click(
      await screen.findByRole('button', { name: "I've joined — verify now" })
    )

    expect(
      await screen.findByText(
        "You haven't joined the required Telegram channel yet."
      )
    ).toBeInTheDocument()
    expect(screen.queryByText('dashboard content')).not.toBeInTheDocument()
  })
})
