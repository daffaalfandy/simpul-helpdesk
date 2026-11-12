import { ApiError, type Problem } from './client'

type ApiResult<T> = { data?: T; error?: unknown; response: Response }

/** Returns the body of a successful openapi-fetch call, or throws an ApiError. */
export async function unwrap<T>(call: Promise<ApiResult<T>>): Promise<T> {
  const { data, error, response } = await call
  if (error !== undefined || !response.ok) throw new ApiError(response, error as Partial<Problem> | undefined)
  return data as T
}
