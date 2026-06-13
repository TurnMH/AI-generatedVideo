export function getApiErrorMessage(error: unknown): string {
  const response = (error as { response?: { data?: { message?: string; error?: string } } })?.response?.data
  return response?.message || response?.error || (error as { message?: string })?.message || ''
}
