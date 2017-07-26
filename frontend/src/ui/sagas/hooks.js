import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import {
  getFormValues,
  isValid
} from 'redux-form'

import apiSaga from 'template-ui/lib/plugins/api/saga'
import consoleTools from 'template-ui/lib/utils/console'

import config from '../config'
import * as actions from '../actions'
import * as selectors from '../selectors'

import auth from './auth'

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
  return {
    routerChanged: [
      auth.authenticateRoute
    ],
    authLogout: auth.logout,
    authLoginSubmit: auth.login,
    authLoginSuccess: auth.loginSuccess,
    authRegisterSubmit: auth.register,
    authRegisterSuccess: auth.registerSuccess,
    apiRequest: Logger('request'),
    apiResponse: Logger('response'),
    apiError: Logger('error')
  }
}

export default Hooks