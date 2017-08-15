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

  // load the current volume list
  function* list() {
    console.log('loading volume list')
    const result = yield call(apis.list.loader)

    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.dir(result)
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
    yield cancel(currentLoopTask)
  }

  return {
    list,
    startLoop,
    stopLoop
  }
}

export default VolumeSagas