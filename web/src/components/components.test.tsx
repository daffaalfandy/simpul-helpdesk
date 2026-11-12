import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { Button } from './Button'
import { TextInput } from './TextInput'

describe('Button', () => {
  it('ignores presses while pending and says so', async () => {
    const onPress = vi.fn()
    render(
      <Button isPending onPress={onPress}>
        Save
      </Button>,
    )
    const button = screen.getByRole('button', { name: /Save/ })
    await userEvent.click(button)
    expect(onPress).not.toHaveBeenCalled()
    expect(button).toHaveAttribute('aria-disabled', 'true')
  })

  it('presses normally', async () => {
    const onPress = vi.fn()
    render(<Button onPress={onPress}>Save</Button>)
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onPress).toHaveBeenCalledOnce()
  })
})

describe('TextInput', () => {
  it('labels the input and links its description', () => {
    render(<TextInput label="Work email" description="We never share it." />)
    const input = screen.getByLabelText('Work email')
    expect(input).toHaveAccessibleDescription('We never share it.')
  })

  it('marks the field invalid and shows the message', () => {
    render(<TextInput label="Password" isInvalid errorMessage="Enter your password." />)
    expect(screen.getByLabelText('Password')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('Enter your password.')).toBeInTheDocument()
  })
})
