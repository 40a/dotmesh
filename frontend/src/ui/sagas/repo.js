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

const RepoSagas = (opts = {}) => {
  if(!opts.apis) throw new Error('repo saga requires a api option')
  const apis = opts.apis
  REQUIRED_APIS.forEach(name => {
    if(!apis[name]) throw new Error(`${name} api required`)
  })

  function* setData(payload) {
    yield put(actions.value.set('repos', payload.Volumes || []))
    yield put(actions.value.set('servers', payload.Servers || []))
  }

  // called if there is an error so the user is not looking at stale data
  function* resetData() {
    yield put(actions.value.set('repos', []))
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

  // make sure we are on page one as soon as they change the search
  function* updateSearch(search) {
    const currentPage = yield select(selectors.repos.pageCurrent)
    if(currentPage>1) {
      yield put(actions.router.redirect('/repos/1'))
    }
    yield put(actions.repos.updateSearch(search))
  }

  return {
    list,
    updateSearch
  }
}

export default RepoSagas
