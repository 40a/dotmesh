import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'
import { delay } from 'redux-saga'

import config from '../config'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'

import tools from '../tools'

const REQUIRED_APIS = [
  'list'
]

const VolumeSagas = (opts = {}) => {
  if(!opts.apis) throw new Error('auth saga requires a api option')
  const apis = opts.apis
  REQUIRED_APIS.forEach(name => {
    if(!apis[name]) throw new Error(`${name} api required`)
  })

  ///////////////////////////////////////
  ///////////////////////////////////////
  // HOOKS

  let currentLoopTask = null

  function* setData(payload) {
    yield put(actions.value.set('volumes', payload.Volumes || []))
    yield put(actions.value.set('servers', payload.Servers || []))
  }

  // called if there is an error so the user is not looking at stale data
  function* resetData() {
    yield put(actions.value.set('volumes', []))
    yield put(actions.value.set('servers', []))
  }

  // load the current volume list
  function* list() {
    const { answer, error } = yield call(apis.list.loader)

    if(error) {
      yield call(resetData)
    }
    else {
      yield call(setData, answer)
    }
  }

  function* listLoop() {
    try {
      while (true) {
        yield call(list)
        yield call(delay, config.volumeLoopInterval)
      }
    } finally {
      if (yield cancelled()) {
        console.log('volume list loop cancelled')
      }
    }
  }

  function* startLoop() {
    console.log('starting loop')
    currentLoopTask = yield fork(listLoop)
  }

  function* stopLoop() {
    console.log('stopping loop')
    if(currentLoopTask) {
      yield cancel(currentLoopTask)  
    }
  }

  return {
    list,
    startLoop,
    stopLoop
  }
}

export default VolumeSagas