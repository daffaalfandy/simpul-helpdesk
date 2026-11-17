import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import type {
  BusinessHoursInput,
  CatalogItemInput,
  ClientInput,
  MemberInvite,
  OrganizationInput,
  PriorityLevel,
  RequesterInput,
  StaffRole,
} from '../../api/client'
import { api } from '../../api/client'
import { clientsQueryOptions, queryKeys, useInvalidate } from '../../api/queries'
import { unwrap } from '../../api/unwrap'

export { clientsQueryOptions }

export const businessHoursQueryOptions = queryOptions({
  queryKey: queryKeys.businessHours,
  queryFn: () => unwrap(api.GET('/business-hours')),
})

export const holidaysQueryOptions = (year: number) =>
  queryOptions({
    queryKey: queryKeys.holidaysForYear(year),
    queryFn: () => unwrap(api.GET('/holidays', { params: { query: { year } } })),
  })

export const holidayPresetsQueryOptions = queryOptions({
  queryKey: queryKeys.holidayPresets,
  queryFn: () => unwrap(api.GET('/holiday-presets')),
})

export const usersQueryOptions = queryOptions({
  queryKey: queryKeys.users,
  queryFn: () => unwrap(api.GET('/users')),
})

export type SaveClient = {
  id?: string
  input: ClientInput
  agentIds: string[]
  /** Agents before editing; the agents call is skipped when the set is unchanged. */
  previousAgentIds: string[]
}

function sameSet(a: readonly string[], b: readonly string[]) {
  return a.length === b.length && a.every((id) => b.includes(id))
}

export function useSaveClient() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: async ({ id, input, agentIds, previousAgentIds }: SaveClient) => {
      let client = id
        ? await unwrap(api.PUT('/clients/{clientId}', { params: { path: { clientId: id } }, body: input }))
        : await unwrap(api.POST('/clients', { body: input }))
      if (!sameSet(agentIds, previousAgentIds)) {
        client = await unwrap(
          api.PUT('/clients/{clientId}/agents', { params: { path: { clientId: client.id } }, body: { userIds: agentIds } }),
        )
      }
      return client
    },
    onSettled: () => invalidate(queryKeys.clients),
  })
}

export function useDeleteClient() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE('/clients/{clientId}', { params: { path: { clientId: id } } })),
    onSuccess: () => invalidate(queryKeys.clients),
  })
}

/** Renames the priorities; every ticket label and report-form card reads the new names. */
export function useSavePriorityLevels() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (levels: PriorityLevel[]) => unwrap(api.PUT('/priorities', { body: levels })),
    onSuccess: (levels) => queryClient.setQueryData(queryKeys.priorities, levels),
  })
}

/** Saves Settings › General; the header, footer and portal read the organisation from the cache. */
export function useUpdateOrganization() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: OrganizationInput) => unwrap(api.PUT('/organization', { body })),
    onSuccess: (org) => queryClient.setQueryData(queryKeys.organization, org),
  })
}

/** The multipart field the API reads the logo from. */
const LOGO_FORM_FIELD = 'file'

export function useUploadLogo() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (file: File) =>
      unwrap(
        api.PUT('/organization/logo', {
          // openapi-typescript types `format: binary` as string; the serializer sends the real File.
          body: { file: file as unknown as string },
          bodySerializer: () => {
            const form = new FormData()
            form.append(LOGO_FORM_FIELD, file, file.name)
            return form
          },
        }),
      ),
    onSuccess: (org) => queryClient.setQueryData(queryKeys.organization, org),
  })
}

export function useRemoveLogo() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/organization/logo')),
    onSuccess: (org) => queryClient.setQueryData(queryKeys.organization, org),
  })
}

export const membersQueryOptions = queryOptions({
  queryKey: queryKeys.members,
  queryFn: () => unwrap(api.GET('/members')),
})

export function useInviteMember() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: MemberInvite) => unwrap(api.POST('/members', { body })),
    onSuccess: () => invalidate(queryKeys.members, queryKeys.users),
  })
}

export function useChangeMemberRole() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: StaffRole }) =>
      unwrap(api.PUT('/members/{userId}/role', { params: { path: { userId } }, body: { role } })),
    onSuccess: () => invalidate(queryKeys.members, queryKeys.users),
  })
}

/** What an admin can do to someone's access from the member list (or a client's requesters). */
/** A client's requesters for the administrator: suspended ones too, so they can be let back in. */
export const clientRequestersAdminQueryOptions = (clientId: string) =>
  queryOptions({
    queryKey: [...queryKeys.clientRequesters(clientId), 'withSuspended'] as const,
    queryFn: () =>
      unwrap(api.GET('/clients/{clientId}/requesters', { params: { path: { clientId }, query: { includeSuspended: true } } })),
  })

/** Invites a requester to a client; they get an email to set their password. */
export function useInviteRequester(clientId: string) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: RequesterInput) =>
      unwrap(api.POST('/clients/{clientId}/requesters', { params: { path: { clientId } }, body })),
    onSuccess: () => invalidate(queryKeys.clients),
  })
}

export type MemberAction = 'suspend' | 'reactivate' | 'invite'

export function useMemberAction() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ userId, action }: { userId: string; action: MemberAction }) =>
      unwrap(api.POST(`/members/{userId}/${action}`, { params: { path: { userId } } })),
    onSuccess: () => invalidate(queryKeys.members, queryKeys.users, queryKeys.clients),
  })
}

export function useUpdateBusinessHours() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: BusinessHoursInput) => unwrap(api.PUT('/business-hours', { body: input })),
    onSuccess: (hours) => queryClient.setQueryData(queryKeys.businessHours, hours),
  })
}

export function useCreateHoliday() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: { date: string; name: string }) => unwrap(api.POST('/holidays', { body })),
    onSuccess: () => invalidate(queryKeys.holidays, queryKeys.holidayPresets),
  })
}

export function useDeleteHoliday() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE('/holidays/{holidayId}', { params: { path: { holidayId: id } } })),
    onSuccess: () => invalidate(queryKeys.holidays, queryKeys.holidayPresets),
  })
}

export function useLoadHolidayPreset() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (year: number) => unwrap(api.POST('/holiday-presets/{year}/load', { params: { path: { year } } })),
    onSuccess: () => invalidate(queryKeys.holidays, queryKeys.holidayPresets),
  })
}

export const catalogItemsQueryOptions = queryOptions({
  queryKey: queryKeys.catalogItems,
  queryFn: () => unwrap(api.GET('/catalog-items')),
})

export function useSaveCatalogItem() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, input }: { id?: string; input: CatalogItemInput }) =>
      id
        ? unwrap(api.PUT('/catalog-items/{catalogItemId}', { params: { path: { catalogItemId: id } }, body: input }))
        : unwrap(api.POST('/catalog-items', { body: input })),
    onSuccess: () => invalidate(queryKeys.catalogItems, queryKeys.intake),
  })
}

export function useDeleteCatalogItem() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE('/catalog-items/{catalogItemId}', { params: { path: { catalogItemId: id } } })),
    onSuccess: () => invalidate(queryKeys.catalogItems, queryKeys.intake),
  })
}
