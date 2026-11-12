function childEnv(source) {
  const env = { ...source }
  for (const key of Object.keys(env)) {
    if (key.toUpperCase().startsWith('APP_')) {
      delete env[key]
    }
  }
  return env
}

module.exports = {
  childEnv
}
