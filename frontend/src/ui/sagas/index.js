import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'
import { delay } from 'redux-saga'
import RouterSaga from 'template-ui/lib/plugins/router/saga'

import apis from '../api'
import * as actions from '../actions'
import config from '../config'

import Hooks from './hooks'
import Auth from './auth'
import Volume from './volume'
import Config from './config'
import Controller from './controller'

const auth = Auth({
  apis: {
    login: apis.authLogin,
    register: apis.authRegister
  }
})

const volume = Volume({
  apis: {
    list: apis.volumeList
  }
})

const configSaga = Config({
  apis: {
    load: apis.configLoad
  }
})

const hooks = Hooks({
  auth,
  volume,
  config: configSaga
})

const router = RouterSaga({
  hooks,
  basepath: config.basepath,
  authenticate: auth.authenticateRoute,
  trigger: (name, payload) => {
    if(process.env.NODE_ENV=='development') {
      console.log('-------------------------------------------');
      console.log('hook')
      console.log(name)
      console.dir(payload)
    }
  }
})

const controllerLoop = Controller({
  
})

function* initialize() {
  yield call(delay, 1)
  yield call(auth.initialize)
  yield call(configSaga.load)
  yield put(actions.value.set('initialized', true))
  yield call(router.initialize)
  yield call(controllerLoop.start)
}

export default function* root() {
  yield all([
    initialize,
    router.main
  ].map(fork))
}
