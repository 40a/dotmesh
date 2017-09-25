import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'
import { delay } from 'redux-saga'

import config from '../config'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'
import tools from '../tools'

const REQUIRED_APIS = [
  'load'
]

const ConfigSagas = (opts = {}) => {
  if(!opts.apis) throw new Error('config saga requires a api option')
  const apis = opts.apis
  REQUIRED_APIS.forEach(name => {
    if(!apis[name]) throw new Error(`${name} api required`)
  })
  
  function* setData(payload) {
    yield put(actions.value.set('config', payload || {}))
  }

  // called if there is an error so the user is not looking at stale data
  function* resetData() {
    yield put(actions.value.set('config', {}))
  }

  // load the current config
  function* load() {
    const { answer, error } = yield call(apis.load.loader)

    if(error) {
      yield call(resetData)
    }
    else {
      tools.devRun(() => {
        console.log('config loaded')
        console.log('-------------------------------------------');
        console.log(JSON.stringify(answer, null, 4))
        console.log('-------------------------------------------');
      })
      yield call(setData, answer)
    }
  }

  return {
    load
  }
}

export default ConfigSagas
