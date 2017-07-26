import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import apis from '../api'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'

function* logout() {
  opts.handleLogout()
}

function* status(action = {}) {
  //const { answer, error } = yield call(apis.authStatus.loader)

  console.log('-------------------------------------------');
  console.log('load status')
  
}

function* login(action = {}) {
  const valid = yield select(selectors.form.authLogin.valid)
  if(!valid) return
  const values = yield select(selectors.form.authLogin.values)

  console.log('-------------------------------------------');
  console.log('-------------------------------------------');
  console.log('-------------------------------------------');
  console.log('try the login')
  console.log(JSON.stringify(values, null, 4))

  /*
  const { answer, error } = yield call(apiSaga, {
    name: 'authLogin',
    actions: actions.login,
    api: apis.login,
    payload: values
  })
  if(error) {
    yield put(routerActions.hook('authLoginError', error))
    return
  }
  const user = yield call(status)
  yield put(routerActions.hook('authLoginSuccess', user))
  return user
  */
}

function* register(action = {}) {

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

const authSagas = {
  logout,
  status,
  login,
  register,
  loginSuccess,
  registerSuccess,
  authenticateRoute
}

export default authSagas