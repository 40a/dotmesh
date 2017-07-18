import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import {
  getFormValues,
  isValid
} from 'redux-form'

import apiSaga from 'template-ui/lib/plugins/api/saga'

import api from '../api'
import * as actions from '../actions'

const Triggers = (opts = {}) => {

  const auth = opts.auth

  function* loginSuccess(user) {
    yield put(actions.router.redirect('/'))
  }

  function* registerSuccess(user) {
    yield put(actions.router.redirect('/'))
  }

  return {
    logout: auth.logout,
    loginSubmit: auth.login,
    registerSubmit: auth.register,
    loginSuccess,
    registerSuccess
  }
}

export default Triggers