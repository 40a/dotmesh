import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'
import { delay } from 'redux-saga'

import config from '../config'
import * as actions from '../actions'
import * as selectors from '../selectors'

import FullStory from './analytics/fullstory'
import Google from './analytics/google'
import Intercom from './analytics/intercom'
import Mixpanel from './analytics/mixpanel'

import tools from '../tools'

const REQUIRED_APIS = [
  
]

const AnalyticsSagas = (opts = {}) => {
  
  const fullstory = FullStory(opts)
  const google = Google(opts)
  const intercom = Intercom(opts)
  const mixpanel = Mixpanel(opts)

  function* setUser(user) {
    yield call(fullstory.setUser, user)
    yield call(google.setUser, user)
    yield call(intercom.setUser, user)
    yield call(mixpanel.setUser, user)
  }

  function* visitPage(path, data) {
    yield call(fullstory.visitPage, path, data)
    yield call(google.visitPage, path, data)
    yield call(intercom.visitPage, path, data)
    yield call(mixpanel.visitPage, path, data)
  }

  function* initialize() {
    yield call(fullstory.initialize)
    yield call(google.initialize)
    yield call(intercom.initialize)
    yield call(mixpanel.initialize)
  }

  return {
    setUser,
    visitPage,
    initialize
  }
}

export default AnalyticsSagas
