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
import { nanoid } from 'nanoid'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { parseEpayChannels, type EpayChannel } from './epay-channels'

type EpayChannelsEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  error?: string
}

export function EpayChannelsEditor(props: EpayChannelsEditorProps) {
  const { t } = useTranslation()
  const id = useId()
  const channels = parseEpayChannels(props.value)
  const [rowState, setRowState] = useState(() => ({
    value: props.value,
    ids: channels.map(() => nanoid()),
  }))
  let rows = rowState
  if (rows.value !== props.value) {
    rows = { value: props.value, ids: channels.map(() => nanoid()) }
    setRowState(rows)
  }
  const updateRows = (nextChannels: EpayChannel[], ids = rows.ids) => {
    const value = JSON.stringify(nextChannels)
    setRowState({ value, ids })
    props.onChange(value)
  }
  const updateChannel = (
    index: number,
    field: keyof EpayChannel,
    value: string
  ) => {
    updateRows(
      channels.map((channel, row) =>
        row === index ? { ...channel, [field]: value } : channel
      )
    )
  }

  return (
    <div className='space-y-4'>
      <p className='text-muted-foreground text-sm'>
        {t('Channel names must contain 1–48 English letters only.')}{' '}
        {t('Epay channel names must be unique (case-sensitive).')}
      </p>
      {channels.map((channel, index) => (
        <fieldset
          key={rows.ids[index]}
          disabled={props.disabled}
          className='space-y-4 rounded-lg border p-4'
        >
          <legend className='px-1 text-sm font-medium'>
            {t('Epay channel {{number}}', { number: index + 1 })}
          </legend>
          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor={`${id}-${index}-name`}>{t('Channel name')}</Label>
              <Input
                id={`${id}-${index}-name`}
                value={channel.name}
                maxLength={48}
                placeholder='achannel'
                autoComplete='off'
                aria-invalid={!!props.error}
                aria-describedby={props.error ? `${id}-error` : undefined}
                onChange={(event) =>
                  updateChannel(index, 'name', event.target.value)
                }
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor={`${id}-${index}-address`}>
                {t('Epay endpoint')}
              </Label>
              <Input
                id={`${id}-${index}-address`}
                value={channel.pay_address}
                placeholder='https://pay.example.com'
                aria-invalid={!!props.error}
                aria-describedby={props.error ? `${id}-error` : undefined}
                onChange={(event) =>
                  updateChannel(index, 'pay_address', event.target.value)
                }
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor={`${id}-${index}-merchant`}>
                {t('Epay merchant ID')}
              </Label>
              <Input
                id={`${id}-${index}-merchant`}
                value={channel.epay_id}
                autoComplete='off'
                aria-invalid={!!props.error}
                aria-describedby={props.error ? `${id}-error` : undefined}
                onChange={(event) =>
                  updateChannel(index, 'epay_id', event.target.value)
                }
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor={`${id}-${index}-key`}>
                {t('Epay secret key')}
              </Label>
              <Input
                id={`${id}-${index}-key`}
                value={channel.epay_key}
                type='password'
                autoComplete='new-password'
                aria-invalid={!!props.error}
                aria-describedby={`${id}-key-help${props.error ? ` ${id}-error` : ''}`}
                onChange={(event) =>
                  updateChannel(index, 'epay_key', event.target.value)
                }
              />
            </div>
          </div>
          <Button
            type='button'
            variant='outline'
            onClick={() =>
              updateRows(
                channels.filter((_, row) => row !== index),
                rows.ids.filter((_, row) => row !== index)
              )
            }
          >
            {t('Remove channel')}
          </Button>
        </fieldset>
      ))}
      {props.error && (
        <p id={`${id}-error`} role='alert' className='text-destructive text-sm'>
          {t(props.error)}
        </p>
      )}
      <p className='text-destructive text-sm'>
        {t(
          'Deleting or renaming channels, or changing their keys, may prevent pending payments from completing.'
        )}
      </p>
      <p id={`${id}-key-help`} className='text-muted-foreground text-sm'>
        {t(
          'Leave the secret blank to keep the saved key for the same channel name. New or renamed channels require a key.'
        )}
      </p>
      <Button
        type='button'
        variant='outline'
        disabled={props.disabled}
        onClick={() =>
          updateRows(
            [
              ...channels,
              { name: '', pay_address: '', epay_id: '', epay_key: '' },
            ],
            [...rows.ids, nanoid()]
          )
        }
      >
        {t('Add Epay channel')}
      </Button>
    </div>
  )
}
