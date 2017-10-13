import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'
import { delay } from 'redux-saga'
import RouterSaga from 'template-ui/lib/plugins/router/saga'

import apis from '../api'
import * as actions from '../actions'
import config from '../config'

import Hooks from './hooks'
import Auth from './auth'
import Repo from './repo'
import Billing from './billing'
import Config from './config'
import Controller from './controller'
import Analytics from './analytics'

const analytics = Analytics({
  apis: {
    
  }
})

const auth = Auth({
  analytics,
  apis: {
    login: apis.authLogin,
    register: apis.authRegister
  }
})

const repo = Repo({
  analytics,
  apis: {
    list: apis.repoList,
    create: apis.repoCreate,
    loadCommits: apis.repoLoadCommits,
    addCollaborator: apis.repoAddCollaborator
  }
})

const billing = Billing({
  analytics,
  apis: {
    submit: apis.billingSubmitPayment
  }
})

const configSaga = Config({
  apis: {
    load: apis.configLoad
  }
})

const hooks = Hooks({
  auth,
  repo,
  billing,
  config: configSaga
})

const controllerLoop = Controller({
  hooks
})

function* onRouteChange() {
  const routerState = yield select(state => state.router)
  yield call(analytics.visitPage, routerState.pathname, routerState)
  yield call(controllerLoop.onRouteChange)
}

const router = RouterSaga({
  hooks,
  basepath: config.basepath,
  authenticate: auth.authenticateRoute,
  onChange: onRouteChange,
  trigger: (name, payload) => {
    if(process.env.NODE_ENV=='development') {
      console.log(`hook: ${name} ${payload && payload.name ? payload.name : ''}`)
      if(payload) console.dir(payload)
    }
  }
})

function* initialize() {
  yield call(delay, 1)
  yield call(analytics.initialize)
  yield call(auth.initialize)
  yield put(actions.value.set('initialized', true))
  yield call(router.initialize)
}

export default function* root() {
  yield all([
    initialize,
    router.main
  ].map(fork))
}
