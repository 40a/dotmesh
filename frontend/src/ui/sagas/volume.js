import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

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

  // load the current volume list
  function* list() {
    const result = yield call(apis.list.loader)

    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.dir(result)
    
  }

  function* startLoop() {
    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.log('starting volume loop')
  }

  function* stopLoop() {
    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.log('stopping volume loop')
  }

  return {
    list,
    startLoop,
    stopLoop
  }
}

export default VolumeSagas