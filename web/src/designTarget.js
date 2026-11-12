export function designTarget(connectionID, schema, driver, kind) {
  return {
    connectionID: String(connectionID || ''),
    schema: String(schema || ''),
    driver: String(driver || ''),
    kind: String(kind || '')
  }
}

export function sameDesignTarget(left, right) {
  return !!left && !!right
    && left.connectionID === right.connectionID
    && left.schema === right.schema
    && left.driver === right.driver
    && left.kind === right.kind
}
