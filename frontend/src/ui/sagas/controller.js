import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel, cancelled } from 'redux-saga/effects'
import { delay } from 'redux-saga'

import config from '../config'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'

import tools from '../tools'

const REQUIRED_OPTS = [
  'hooks'
]

/*

  loop every second and run the hooks names as `controlLoopHooks` from the router results
  
*/

const ControllerLoop = (opts = {}) => {
  REQUIRED_OPTS.forEach(name => {
    if(!opts[name]) throw new Error(`${name} opt required`)
  })

  const hooks = opts.hooks

  // reference to the saga so we can cancel it
  let currentLoopTask = null

  // block whilst we are doing the saga itself
  function* singleLoop() {
    const routerResults = yield select(state => state.router.result || {})
    const sagaName = routerResults.controlLoopSaga
    const saga = sagaName && hooks[sagaName] ? hooks[sagaName] : null
    let result = null
    if(saga) {
      result = yield call(saga)
    }
    return result
  }

  function* runLoop() {
    try {
      while (true) {
        yield call(singleLoop)
        yield call(delay, config.controlLoopInterval)
      }
    } finally {
      if (yield cancelled()) {
        console.log('controller loop cancelled')
      }
    }
  }

  function* start() {
    currentLoopTask = yield fork(runLoop)
  }

  function* stop() {
    if(currentLoopTask) {
      yield cancel(currentLoopTask)  
    }
  }

  return {
    start,
    stop
  }
}

export default ControllerLoop