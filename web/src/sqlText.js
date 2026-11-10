import { quoteIdent } from './sqlEditor.js'

export function selectStar(driver, schema, table) {
  const name = quoteIdent(driver, table)
  if (!schema) {
    return `select * from ${name}`
  }
  return `select * from ${quoteIdent(driver, schema)}.${name}`
}
