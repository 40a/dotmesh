import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'
import { delay } from 'redux-saga'
import RouterSaga from 'template-ui/lib/plugins/router/saga'

import apis from '../api'
import * as actions from '../actions'
import config from '../config'

import Hooks from './hooks'
import Auth from './auth'

const auth = Auth({
  apis: {
    login: apis.authLogin,
    register: apis.authRegister
  }
})

const hooks = Hooks({
  auth
})

const router = RouterSaga({
  hooks,
  basepath: config.basepath
})

function* initialize() {
  yield call(delay, 1)
  yield all([
    call(auth.initialize)
  ])
  yield put(actions.value.set('initialized', true))
  yield call(router.initialize)
}

export default function* root() {
  yield all([
    initialize,
    router.main
  ].map(fork))
}