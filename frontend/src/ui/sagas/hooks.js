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

const Hooks = (opts = {}) => {
  if(!opts.auth) throw new Error('auth opt required for hooks')
  const auth = opts.auth
  return {
    routerChanged: [
      auth.authenticateRoute
    ],
    authLogout: auth.logout,
    authLoginSubmit: auth.loginSubmit,
    authLoginSuccess: auth.loginSuccess,
    authRegisterSubmit: auth.registerSubmit,
    authRegisterSuccess: auth.registerSuccess,
    apiRequest: Logger('request'),
    apiResponse: Logger('response'),
    apiError: Logger('error')
  }
}

export default Hooks