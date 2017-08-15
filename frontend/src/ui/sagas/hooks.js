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
  if(!opts.volume) throw new Error('volume opt required for hooks')
  const auth = opts.auth
  const volume = opts.volume
  return {

    // router hooks for authenticating route
    routerChanged: [
      auth.authenticateRoute
    ],

    // auth hooks for register/login
    authLogout: auth.logout,
    authLoginSubmit: auth.loginSubmit,
    authLoginSuccess: auth.loginSuccess,
    authRegisterSubmit: auth.registerSubmit,
    authRegisterSuccess: auth.registerSuccess,

    // volume
    volumeList: volume.list,
    volumeStartLoop: volume.startLoop,
    volumeStopLoop: volume.stopLoop,

    // generic hooks for logging
    apiRequest: Logger('request'),
    apiResponse: Logger('response'),
    apiError: Logger('error')
  }
}

export default Hooks