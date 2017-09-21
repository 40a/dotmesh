import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'
import { delay } from 'redux-saga'

import config from '../config'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'

import listUtils from '../utils/list'
import tools from '../tools'

const REQUIRED_APIS = [
  'list',
  'create'
]

const RepoSagas = (opts = {}) => {
  if(!opts.apis) throw new Error('repo saga requires a api option')
  const apis = opts.apis
  REQUIRED_APIS.forEach(name => {
    if(!apis[name]) throw new Error(`${name} api required`)
  })

  function* setData(payload) {
    let repos = payload.Volumes
    let servers = payload.Servers
    repos = listUtils.sortObjectList(repos, selectors.repo.name)
    yield put(actions.value.set('reposLoaded', true))
    yield put(actions.value.set('repos', repos || []))
    yield put(actions.value.set('servers', servers || []))
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
      yield put(actions.router.redirect('/repos/page/1'))
    }
    yield put(actions.repos.updateSearch(search))
  }

  function* updatePage(page) {
    yield put(actions.router.redirect(`/repos/page/${page}`))
  }

  function* formInitialize() {
    yield put(actions.forms.reset('repo'))
  }

  function* formSubmit() {
    const isValid = yield select(state => selectors.form.repo.valid(state))
    const values = yield select(state => selectors.form.repo.values(state))

    if(!isValid) return

    yield put(actions.value.set('repoFormLoading', true))
    const Name = values.Name
    const Namespace = yield select(selectors.auth.name)

    const payload = {
      Namespace,
      Name
    }

    // load the repos so we can see if the one they are adding exists already
    yield call(list)

    const exists = yield select(state => selectors.repos.exists(state, payload))

    if(exists) {
      yield put(actions.value.set('repoFormLoading', false))
      yield put(actions.application.setMessage(`repo with name: ${Namespace} / ${Name} already exists`))
      return
    }

    const { answer, error } = yield call(apis.create.loader, payload)
    
    if(error) {
      yield put(actions.value.set('repoFormLoading', false))
      yield put(actions.application.setMessage(error.toString()))
      return
    }

    yield put(actions.value.set('repoFormLoading', false))
    yield put(actions.application.setMessage(`repo ${Namespace} / ${Name} created`))
    yield put(actions.router.redirect('/repos'))
  }

  return {
    list,
    updateSearch,
    updatePage,
    formInitialize,
    formSubmit
  }
}

export default RepoSagas
