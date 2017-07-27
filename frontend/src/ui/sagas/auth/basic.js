import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import apis from '../../api'
import forms from '../../forms'
import * as actions from '../../actions'
import * as selectors from '../../selectors'
import authUtils from '../../utils/auth'

// HELPERS

// const encoded = authUtils.encodeCredentials(username, password)

// save the encoded user/password into state so we can pass it along
// with every rpc reuqest
function* saveCredentials(encoded) {
  yield put(actions.value.set('credentials', encoded))
  return encoded
}

function* loadCredentials() {
  const credentials = yield select(selectors.value('credentials'))
  return credentials
}

// HOOKS

// 
function* status(encodedCredentials) {
  //const { answer, error } = yield call(apis.authStatus.loader)

  // if we are not passed credentials then used the stashed ones
  if(!encodedCredentials) {
    encodedCredentials = yield call(loadCredentials)
  }
  
  // if we don't have credentials here then are not defo not logged in
  if(!encodedCredentials) return

  const { answer, error } = yield call(apis.authStatus.loader, {
    headers: authUtils.getHeaders(encodedCredentials)
  })

  if(error) return false
  return true
}


function* login() {
  const valid = yield select(selectors.form.authLogin.valid)
  if(!valid) return
  const values = yield select(selectors.form.authLogin.values)

  const encodedCredentials = authUtils.encodeCredentials(values.username, values.password)

  const loggedIn = yield call(status, encodedCredentials)

  if(!loggedIn) {
    yield put(actions.router.hook('authLoginError', 'incorrect details'))
    return
  }
  else {
    const user = {
      username: values.username
    }
    yield put(actions.value.set('user', user))
    yield call(saveCredentials, encodedCredentials)
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
  logout,
  status,
  login,
  register,
  loginSuccess,
  registerSuccess,
  authenticateRoute
}

export default authSagas