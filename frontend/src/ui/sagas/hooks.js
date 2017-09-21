import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import consoleTools from 'template-ui/lib/utils/console'

import config from '../config'

const Logger = (type) => {
  function* logger(req) {
    consoleTools.devRun(() => {
      console.log(`api ${type}: ${req.name}`)
      console.dir(req)
    })
  }
  return logger
}

const REQUIRED_SAGA_GROUPS = [
  'auth',
  'repo',
  'billing',
  'config'
]

const Hooks = (opts = {}) => {
  REQUIRED_SAGA_GROUPS.forEach(name => {
    if(!opts[name]) throw new Error(`${name} saga group required`)
  })

  const auth = opts.auth
  const repo = opts.repo
  const billing = opts.billing
  const config = opts.config

  return {

    // auth hooks for register/login
    authLogout: auth.logout,
    authLoginError: () => {},
    authLoginSubmit: auth.loginSubmit,
    authLoginSuccess: auth.loginSuccess,
    authRegisterSubmit: auth.registerSubmit,
    authRegisterSuccess: auth.registerSuccess,

    // repo
    repoList: repo.list,
    repoUpdateSearch: repo.updateSearch,
    repoUpdatePage: repo.updatePage,
    repoFormSubmit: repo.formSubmit,
    repoFormInitialize: repo.formInitialize,

    // billing
    billingTokenReceived: billing.tokenReceived,

    // config
    configLoad: config.load,

    // generic hooks for logging
    apiRequest: Logger('request'),
    apiResponse: Logger('response'),
    apiError: Logger('error')
  }
}

export default Hooks
