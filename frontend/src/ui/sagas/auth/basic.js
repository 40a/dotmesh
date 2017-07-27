import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import apis from '../../api'
import forms from '../../forms'
import * as actions from '../../actions'
import * as selectors from '../../selectors'

// HELPERS

// const encoded = authUtils.encodeCredentials(username, password)

// save the encoded user/password into state so we can pass it along
// with every rpc reuqest
function* putCredentials(credentials) {
  yield put(actions.value.set('user', credentials))
  return credentials
}

function* selectCredentials() {
  const credentials = yield select(selectors.value('user'))
  return credentials
}

const loadCachedCredentials = () => {
  const localValue = localStorage.getItem('user')
  return localValue ?
    JSON.parse(localValue.toString()) :
    null
}

const saveCachedCredentials = (credentials) => {
  localStorage.getItem('user', JSON.stringify(credentials))
}

// HOOKS
function* initialize() {
  // do we have credentials in local storage?
  const cachedCredentials = loadCachedCredentials()
  if(cachedCredentials) {
    yield call(login, cachedCredentials)
  }
}

function* login(credentials) {
  if(!credentials) {
    const valid = yield select(selectors.form.authLogin.valid)
    if(!valid) return
    credentials = yield select(selectors.form.authLogin.values)
  }
  if(!credentials) return false
  const { answer, error } = yield call(apis.authLogin.loader, {
    credentials
  })
  if(error) {
    yield put(actions.router.hook('authLoginError', 'incorrect details'))
    return
  }
  else {
    yield call(putCredentials, credentials)

    // TODO: if remember me then cache credentials
    yield put(actions.router.hook('authLoginSuccess', user))
    return user
  }
}

function* register() {

  /*
  const valid = yield select(isValid('register'))
  if(!valid) return
  const values = yield select(getFormValues('register'))

  const { answer, error } = yield call(apiSaga, {
    name: 'authRegister',
    actions: actions.register,
    api: apis.register,
    payload: values
  })

  if(error) {
    yield put(routerActions.hook('authRegisterError', user))
    return
  }

  const user = yield call(status)
  yield put(routerActions.hook('authRegisterSuccess', user))
  return user
  */
}

function* loginSuccess(user) {
  //yield put(actions.router.redirect('/dashboard'))
  alert('logged in')
}

function* registerSuccess(user) {
  //yield put(actions.router.redirect('/dashboard'))
  alert('registered')
}

function* authenticateRoute() {
  const userSetting = yield select(state => selectors.router.firstValue(state, 'user'))
  // this route has no opinion about the user
  if(typeof(userSetting) != 'boolean') return
  const user = yield select(state => selectors.valueSelector(state, 'user'))
  const hasUser = user ? true : false
  const isRouteAuthenticated = hasUser == userSetting
  if(!isRouteAuthenticated) {
    yield put(actions.router.push(config.authRedirect || '/'))
  }
}


function* logout() {
  yield put(actions.value.set(CREDENTIALS_VALUE_NAME), '')
  console.log('-------------------------------------------');
  console.log('logged out')
}


const authSagas = {
  initialize,
  logout,
  status,
  login,
  register,
  loginSuccess,
  registerSuccess,
  authenticateRoute
}

export default authSagas