import { MCP_SERVER_NAME } from '../../lib/constants'

/** JSON indentation in the config snippet. */
const JSON_INDENT = 2

/**
 * The settings an MCP client needs, for one token (or a placeholder). The command and the
 * config keys are the clients' own syntax, so they aren't translated.
 */
export function snippetsFor(mcpUrl: string, token: string) {
  const authorization = `Bearer ${token}`
  return {
    claudeCode: `claude mcp add --transport http ${MCP_SERVER_NAME} ${mcpUrl} --header "Authorization: ${authorization}"`,
    json: JSON.stringify({ mcpServers: { [MCP_SERVER_NAME]: { url: mcpUrl, headers: { Authorization: authorization } } } }, null, JSON_INDENT),
  }
}
