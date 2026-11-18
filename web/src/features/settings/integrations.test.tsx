import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import { renderApp } from '../../test/renderApp'
import { DEMO_SECRET, integrationsApi, REPO_IDS } from '../../test/integrationsApi'
import { PROJECT_IDS, projectsApi } from '../../test/projectsApi'
import { state, USERS } from '../../test/server'

const ADMIN = USERS['admin@demo-vendor.test'] ?? null

function card(name: string) {
  const heading = screen.getByRole('heading', { name, level: 3 })
  const section = heading.closest('section')
  if (!section) throw new Error(`no card ${name}`)
  return within(section)
}

describe('Settings › GitHub / GitLab', () => {
  beforeEach(() => {
    state.me = ADMIN
  })

  it('shows each provider, its repositories and how their webhooks are doing', async () => {
    renderApp('/settings/integrations')
    expect(await screen.findByRole('heading', { name: 'GitHub / GitLab', level: 2 })).toBeInTheDocument()
    const github = card('GitHub')
    expect(github.getByText('Connected')).toBeInTheDocument()
    expect(github.getByRole('link', { name: /vendor-id\/eoffice-jambi/ })).toHaveAttribute('href', 'https://github.com/vendor-id/eoffice-jambi')
    expect(github.getByRole('link', { name: 'e-Office' })).toHaveAttribute('href', `/projects/${PROJECT_IDS.eOffice}`)
    const gitlab = card('GitLab')
    expect(gitlab.getByText('Needs attention')).toBeInTheDocument()
    expect(gitlab.getByText(/gitlab\.vendor\.co\.id\/gov\/sipd-connector/)).toBeInTheDocument()
    expect(gitlab.getByText(/Refused .*: the secret doesn't match/)).toBeInTheDocument()
    expect(gitlab.getByText('No projects read code from it yet.')).toBeInTheDocument()

    const rules = screen.getByRole('table', { name: 'Commit message rules' })
    expect(within(rules).getByText('Fixes HD-24871: revert TTL')).toBeInTheDocument()
    expect(within(rules).getByText(/commits never change a status\./)).toBeInTheDocument()
    // A commit reaching the Definition of Done's branch ticks it (10b), and still moves nothing.
    expect(within(rules).getByText(/the commit ticks "Code reached a branch"/)).toBeInTheDocument()

    const log = card('Recent events')
    expect(await log.findByText(/Push a1f9c2e “Fix HD-24871: revert SSO token TTL” on fix\/HD-24871-sso → linked to/)).toBeInTheDocument()
    expect(log.getByRole('link', { name: 'HD-24871' })).toHaveAttribute('href', '/tickets/HD-24871')
    expect(log.getByText(/Push da15608/)).toHaveTextContent('linked to HD-24826')
    expect(log.getByText(/no such ticket, ignored/)).toHaveTextContent('mentioned HD-99999')
    expect(log.queryByRole('link', { name: 'HD-99999' })).not.toBeInTheDocument()
  })

  it('reads a commit message the way the connector does', async () => {
    const user = userEvent.setup()
    renderApp('/settings/integrations')
    const tester = card(await screen.findByText('Try a commit message').then((h) => h.textContent ?? ''))
    const result = tester.getByRole('status')
    // Keywords only link: the ticket's status doesn't change.
    expect(await within(result).findByText('Linked · HD-24871')).toBeInTheDocument()
    expect(within(result).getByText("Adds the commit to the ticket's code activity. The ticket's status doesn't change.")).toBeInTheDocument()

    await user.click(tester.getByRole('button', { name: 'Closes HD-24830' }))
    expect(await within(result).findByText('Linked · HD-24830')).toBeInTheDocument()

    await user.click(tester.getByRole('button', { name: 'update readme' }))
    expect(await within(result).findByText('No ticket found')).toBeInTheDocument()

    const input = tester.getByRole('textbox', { name: 'Message' })
    await user.clear(input)
    await user.type(input, 'refs HD-1 and HD-24859')
    expect(await within(result).findByText('Linked · HD-1, HD-24859')).toBeInTheDocument()
    expect(within(result).getByText('No ticket HD-1 yet, so it would be logged and ignored.')).toBeInTheDocument()
  })

  it('connects a repository and shows its webhook secret once', async () => {
    const user = userEvent.setup()
    renderApp('/settings/integrations')
    await user.click(await screen.findByRole('button', { name: 'Add a GitHub repository' }))
    const dialog = await screen.findByRole('dialog', { name: 'Add a GitHub repository' })
    await user.click(within(dialog).getByRole('button', { name: 'Add repository' }))
    expect(await within(dialog).findByText("Enter the repository's address.")).toBeInTheDocument()

    await user.type(within(dialog).getByRole('textbox', { name: 'Repository address' }), 'https://github.com/vendor-id/web-bintan')
    await user.click(within(dialog).getByRole('checkbox', { name: /^Email/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Add repository' }))

    const setup = await screen.findByRole('dialog', { name: 'Add the webhook to vendor-id/web-bintan' })
    expect(within(setup).getByText('http://localhost:5173/api/v1/webhooks/r-3')).toBeInTheDocument()
    expect(within(setup).getByText(DEMO_SECRET)).toBeInTheDocument()
    expect(within(setup).getByText('application/json')).toBeInTheDocument()
    expect(within(setup).getByRole('button', { name: 'Copy the Secret' })).toBeInTheDocument()
    expect(within(setup).queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()
    expect(integrationsApi.requests.at(-1)?.body).toEqual({
      provider: 'github',
      url: 'https://github.com/vendor-id/web-bintan',
      projectIds: [PROJECT_IDS.email],
    })

    await user.click(within(setup).getByRole('button', { name: 'Done' }))
    expect(await screen.findByText(/vendor-id\/web-bintan connected/)).toBeInTheDocument()
    expect(card('GitHub').getByText('Waiting for the first event')).toBeInTheDocument()
  })

  it('makes a new secret, edits projects and disconnects', async () => {
    const user = userEvent.setup()
    const eOfficeProject = projectsApi.projects.find((p) => p.id === PROJECT_IDS.eOffice)
    if (eOfficeProject) {
      eOfficeProject.repository = {
        id: REPO_IDS.eOffice, provider: 'github', fullName: 'vendor-id/eoffice-jambi', webUrl: 'https://github.com/vendor-id/eoffice-jambi',
      }
    }
    renderApp('/settings/integrations')
    await user.click(await screen.findByRole('button', { name: 'Make a new secret for gov/sipd-connector' }))
    const confirm = await screen.findByRole('alertdialog', { name: 'Make a new secret for gov/sipd-connector?' })
    await user.click(within(confirm).getByRole('button', { name: 'Make a new secret' }))
    const setup = await screen.findByRole('dialog', { name: 'New secret for gov/sipd-connector' })
    expect(within(setup).getByText('Secret token')).toBeInTheDocument()
    await user.click(within(setup).getByRole('button', { name: 'Done' }))
    expect(await screen.findByText('New secret for gov/sipd-connector made.')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Edit the projects of gov/sipd-connector' }))
    const edit = await screen.findByRole('dialog', { name: 'Projects reading gov/sipd-connector' })
    const eOffice = within(edit).getByRole('checkbox', { name: /^e-Office/ })
    expect(within(edit).getByText('Dinas Kominfo Jambi. Reads vendor-id/eoffice-jambi now; it moves here.')).toBeInTheDocument()
    await user.click(eOffice)
    await user.click(within(edit).getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(integrationsApi.requests.at(-1)).toEqual({
        method: 'PATCH',
        path: `/api/v1/repositories/${REPO_IDS.sipd}`,
        body: { projectIds: [PROJECT_IDS.eOffice] },
      }),
    )

    await user.click(await screen.findByRole('button', { name: 'Disconnect gov/sipd-connector' }))
    const remove = await screen.findByRole('alertdialog', { name: 'Disconnect gov/sipd-connector?' })
    await user.click(within(remove).getByRole('button', { name: 'Disconnect' }))
    expect(await screen.findByText('gov/sipd-connector disconnected.')).toBeInTheDocument()
    expect(card('GitLab').getByText('Not set up')).toBeInTheDocument()
  })
})
