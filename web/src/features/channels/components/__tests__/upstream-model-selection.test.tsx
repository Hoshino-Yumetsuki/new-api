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
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'

import { fetchModels } from '../../api'
import { channelSchema } from '../../types'
import { ChannelsProvider, useChannels } from '../channels-provider'
import { FetchModelsDialog } from '../dialogs/fetch-models-dialog'
import { UpstreamModelSelection } from '../upstream-model-selection'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

test.each([
  {
    result: 'nonempty',
    models: ['gpt-existing', 'gpt-new'],
    removedCount: 1,
    savedModels: ['gpt-existing', 'alias', 'gpt-new'],
  },
  {
    result: 'empty',
    models: [],
    removedCount: 2,
    savedModels: ['alias'],
  },
])(
  'removed models stay available after batch deselection with a $result upstream list and only checked models are saved',
  async ({ models, removedCount, savedModels }) => {
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: models },
    })
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const select = vi.fn()
    const close = vi.fn()
    const user = userEvent.setup()
    const view = render(
      <QueryClientProvider client={client}>
        <ChannelsProvider>
          <FetchModelsDialog
            open
            onOpenChange={close}
            onModelsSelected={select}
            existingModelsOverride={['gpt-existing', 'alias', 'manual-model']}
            redirectSourceModels={['alias']}
            customFetcher={async () =>
              (
                await fetchModels({
                  type: 1,
                  base_url: 'https://example.com',
                  key: 'test-key',
                })
              ).data ?? []
            }
          />
        </ChannelsProvider>
      </QueryClientProvider>
    )
    const removedTab = await screen.findByRole('tab', {
      name: `Removed Models (${removedCount})`,
    })
    if (models.length > 0) {
      await user.click(screen.getByRole('checkbox', { name: 'gpt-new' }))
    }
    await user.click(removedTab)
    expect(
      screen.queryByRole('checkbox', { name: 'alias' })
    ).not.toBeInTheDocument()
    await user.click(
      screen.getByRole('checkbox', { name: 'Select all models in Removed' })
    )
    expect(removedTab).toHaveAttribute('aria-selected', 'true')
    const manualModel = screen.getByRole('checkbox', { name: 'manual-model' })
    expect(manualModel).not.toBeChecked()
    await user.click(manualModel)
    expect(manualModel).toBeChecked()
    await user.click(manualModel)
    expect(manualModel).not.toBeChecked()
    expect(
      screen.getByRole('tab', { name: `Removed Models (${removedCount})` })
    ).toHaveAttribute('aria-selected', 'true')
    expect(select).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Save Models' }))
    expect(select).toHaveBeenCalledWith(savedModels)
    expect(close).toHaveBeenCalledWith(false)
    view.unmount()
    client.clear()
  }
)

function SavedChannelModelPicker(props: { mapping: string | null }) {
  const { open, setOpen, setCurrentRow } = useChannels()
  return (
    <>
      <Button
        onClick={() => {
          setCurrentRow(
            channelSchema.parse({
              id: 42,
              type: 1,
              key: '',
              status: 1,
              name: 'Redirect channel',
              created_time: 0,
              test_time: 0,
              response_time: 0,
              balance_updated_time: 0,
              models: 'alias,manual-model',
              model_mapping: props.mapping,
            })
          )
          setOpen('fetch-models')
        }}
      >
        Fetch channel models
      </Button>
      <FetchModelsDialog
        open={open === 'fetch-models'}
        onOpenChange={(value) => !value && setOpen(null)}
      />
    </>
  )
}

test.each([
  {
    scenario: 'a saved redirect without its target in channel models',
    mapping: '{"alias":"gpt-upstream"}',
    models: ['gpt-upstream', 'gpt-new'],
    existingCount: 1,
    removedCount: 1,
  },
  {
    scenario: 'a saved redirect with an empty upstream list',
    mapping: '{"alias":"gpt-upstream"}',
    models: [],
    existingCount: 0,
    removedCount: 1,
  },
  {
    scenario: 'no saved redirect',
    mapping: null,
    models: ['gpt-upstream', 'gpt-new'],
    existingCount: 0,
    removedCount: 2,
  },
  {
    scenario: 'malformed saved redirect JSON',
    mapping: '{',
    models: ['gpt-upstream', 'gpt-new'],
    existingCount: 0,
    removedCount: 2,
  },
])(
  '$scenario classifies upstream models and only removes explicitly deselected models on save',
  async ({ mapping, models, existingCount, removedCount }) => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: models },
    })
    const save = vi.spyOn(api, 'put').mockResolvedValue({
      data: { success: true },
    })
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const user = userEvent.setup()
    const view = render(
      <QueryClientProvider client={client}>
        <ChannelsProvider>
          <SavedChannelModelPicker mapping={mapping} />
        </ChannelsProvider>
      </QueryClientProvider>
    )

    await user.click(
      screen.getByRole('button', { name: 'Fetch channel models' })
    )
    const removedTab = await screen.findByRole('tab', {
      name: `Removed Models (${removedCount})`,
    })
    expect(
      screen.getByRole('tab', {
        name: `New Models (${models.length - existingCount})`,
      })
    ).toBeVisible()
    const existingTab = screen.getByRole('tab', {
      name: `Existing Models (${existingCount})`,
    })
    if (existingCount > 0) {
      await user.click(existingTab)
      expect(
        screen.getByRole('checkbox', { name: 'gpt-upstream' })
      ).not.toBeChecked()
    }
    await user.click(removedTab)
    if (removedCount === 1) {
      expect(
        screen.queryByRole('checkbox', { name: 'alias' })
      ).not.toBeInTheDocument()
    } else {
      expect(screen.getByRole('checkbox', { name: 'alias' })).toBeChecked()
    }
    await user.click(screen.getByRole('checkbox', { name: 'manual-model' }))
    await user.click(screen.getByRole('button', { name: 'Save Models' }))
    expect(save).toHaveBeenCalledWith(
      '/api/channel/',
      { id: 42, models: 'alias' },
      expect.anything()
    )
    view.unmount()
    client.clear()
  }
)

function InlineSelection() {
  const [selected, setSelected] = useState(['manual-model'])
  return (
    <>
      <UpstreamModelSelection
        models={['gpt-one', 'gpt-two', 'another-model']}
        selected={selected}
        existingModels={[]}
        showChanges={false}
        onChange={setSelected}
      />
      <output aria-label='Selected models'>{selected.join(',')}</output>
    </>
  )
}

test('category headers keep a transparent background while expanding and collapsing without changing selection', async () => {
  const user = userEvent.setup()
  render(<InlineSelection />)
  const header = screen.getByRole('button', { name: /^OpenAI \(2\)/ })

  expect(header).toHaveAttribute('aria-expanded', 'true')
  expect(header).toHaveClass(
    'aria-expanded:bg-transparent',
    'hover:bg-transparent',
    'dark:hover:bg-transparent'
  )
  await user.click(header)
  expect(header).toHaveAttribute('aria-expanded', 'false')
  expect(
    screen.queryByRole('checkbox', { name: 'gpt-one' })
  ).not.toBeInTheDocument()
  await user.keyboard('{Enter}')
  expect(header).toHaveAttribute('aria-expanded', 'true')
  expect(screen.getByRole('checkbox', { name: 'gpt-one' })).toBeVisible()
  expect(header).toHaveFocus()
  expect(screen.getByLabelText('Selected models')).toHaveTextContent(
    'manual-model'
  )
})

test('the matching-model action only appears for a nonblank search and disappears when cleared', async () => {
  const user = userEvent.setup()
  render(<InlineSelection />)
  const search = screen.getByRole('textbox', { name: 'Search models...' })
  expect(
    screen.queryByRole('button', { name: 'Select all matching models' })
  ).not.toBeInTheDocument()

  await user.type(search, '  ')
  expect(
    screen.queryByRole('button', { name: 'Select all matching models' })
  ).not.toBeInTheDocument()

  await user.type(search, 'gpt-')
  expect(
    screen.getByRole('button', { name: 'Select all matching models' })
  ).toBeEnabled()
  await user.clear(search)
  expect(
    screen.queryByRole('button', { name: 'Select all matching models' })
  ).not.toBeInTheDocument()

  await user.type(search, 'missing')
  expect(
    screen.getByRole('button', { name: 'Select all matching models' })
  ).toBeDisabled()
})

test('selecting search results adds only matching models and retains the manual selection', async () => {
  const user = userEvent.setup()
  render(<InlineSelection />)
  await user.type(
    screen.getByRole('textbox', { name: 'Search models...' }),
    'gpt-'
  )
  await user.click(
    screen.getByRole('button', { name: 'Select all matching models' })
  )
  expect(screen.getByLabelText('Selected models')).toHaveTextContent(
    'manual-model,gpt-one,gpt-two'
  )
  expect(
    screen.queryByRole('checkbox', { name: 'another-model' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('checkbox', { name: 'gpt-one' }))
  expect(screen.getByLabelText('Selected models')).toHaveTextContent(
    'manual-model,gpt-two'
  )
})

test('the redirect action appears per model only when a handler is provided and reports the model without toggling it', async () => {
  const user = userEvent.setup()
  const onRedirect = vi.fn()
  const view = render(
    <UpstreamModelSelection
      models={['gpt-one']}
      selected={[]}
      existingModels={[]}
      showChanges={false}
      onChange={() => undefined}
    />
  )
  expect(
    screen.queryByRole('button', { name: 'Redirect gpt-one' })
  ).not.toBeInTheDocument()

  view.rerender(
    <UpstreamModelSelection
      models={['gpt-one']}
      selected={[]}
      existingModels={[]}
      showChanges={false}
      onChange={() => undefined}
      onRedirectModel={onRedirect}
    />
  )
  await user.click(screen.getByRole('button', { name: 'Redirect gpt-one' }))
  expect(onRedirect).toHaveBeenCalledWith('gpt-one')
  expect(screen.getByRole('checkbox', { name: 'gpt-one' })).not.toBeChecked()
})

test('models published under another name show that name as a badge next to the row', () => {
  render(
    <UpstreamModelSelection
      models={['gpt-4o-all', 'o3']}
      selected={[]}
      existingModels={[]}
      showChanges={false}
      onChange={() => undefined}
      aliases={{ 'gpt-4o-all': 'gpt-4o' }}
    />
  )
  expect(screen.getByLabelText('Published as gpt-4o')).toHaveTextContent(
    'gpt-4o'
  )
  expect(screen.queryByLabelText(/Published as o3/)).not.toBeInTheDocument()
})

test('long model names remain fully labeled and selectable in a container-responsive list', async () => {
  const user = userEvent.setup()
  const model = 'gpt-4.1-mini-deployment-with-a-long-region-name-2025-04-14'
  const onChange = vi.fn()
  render(
    <UpstreamModelSelection
      models={[model]}
      selected={[]}
      existingModels={[]}
      showChanges={false}
      onChange={onChange}
    />
  )
  const checkbox = screen.getByRole('checkbox', { name: model })
  const category = checkbox.closest('[data-slot="collapsible"]')
  expect(category).toHaveClass('@container')
  const grid = checkbox.parentElement?.parentElement
  expect(grid).toHaveClass('@lg:grid-cols-2')
  expect(grid).not.toHaveClass('sm:grid-cols-2')
  expect(screen.getByText(model)).toBeVisible()
  await user.click(checkbox)
  expect(onChange).toHaveBeenCalledWith([model])
})
