import { take, put, call, fork, select, all, takeLatest, takeEvery } from 'redux-saga/effects'

import LocalStorage from '../utils/localStorage'

import config from '../config'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'

import tools from '../tools'

const REQUIRED_APIS = [
  'login',
  'register'
]

const AuthSagas = (opts = {}) => {
  if(!opts.apis) throw new Error('auth saga requires a api option')
  const apis = opts.apis
  REQUIRED_APIS.forEach(name => {
    if(!apis[name]) throw new Error(`${name} api required`)
  })

  ///////////////////////////////////////
  ///////////////////////////////////////
  // redux state

  // write the given credentials to the value reducer
  function* reduceCredentials(credentials) {
    yield put(actions.auth.setUser(credentials))
    return credentials
  }

  // get the current credentials from the value reducer
  function* selectCredentials() {
    const credentials = yield select(selectors.auth.user)
    return credentials
  }

  // load credentials saved in local storage (because 'remember me')
  const loadCachedCredentials = () => {
    const localValue = LocalStorage.load(config.userLocalStorageName, true)
    if(localValue) {
      tools.logger(`found locally cached credentials`)
      console.dir(localValue)
    }
    return localValue && localValue.username ?
      localValue :
      null
  }

  ///////////////////////////////////////
  ///////////////////////////////////////
  // local storage state

  // save the given credentials to local storage (because 'remember me')
  const saveCachedCredentials = (credentials) => LocalStorage.save(config.userLocalStorageName, credentials, true)
  const deleteCachedCredentials = () => LocalStorage.del(config.userLocalStorageName)

  ///////////////////////////////////////
  ///////////////////////////////////////
  // HOOKS

  // if we have saved credentials (because 'remember me')
  // then use them to login
  function* initialize() {
    const cachedCredentials = loadCachedCredentials()
    if(cachedCredentials) {
      yield call(login, cachedCredentials, true)
    }
  }

  // given some credentials - login
  function* login(credentials, headless) {
    if(!credentials) throw new Error('credentials required for login saga')

    // save the credentials to the store
    yield call(reduceCredentials, credentials)

    // run the login api
    const result = yield call(apis.login.loader, {
      credentials
    })
    const error = result.error
    const loggedIn = result.answer

    if(error || loggedIn !== true) {
      if(!headless) {
        yield put(actions.router.hook('authLoginError', 'incorrect details'))  
      }
      return
    }
    else {
      
      // save credentials to local storage
      saveCachedCredentials(credentials)

      if(!headless) {
        // save the credentials to local storage so upon re-opening browser we are authenticated
        yield put(actions.router.hook('authLoginSuccess', credentials))  
      }
      
      return credentials
    }
  }

  // called when the user clicks the submit button
  // grab the values from the 'authLogin' form and send them to the 'login' saga
  function* loginSubmit() {
    const credentials = yield call(formValuesIfValid, 'authLogin')
    if(credentials) {
      yield call(login, credentials)  
    }
  }

  function* register(credentials) {
    if(!credentials) throw new Error('credentials required for register saga')

    const sendCredentials = {
      email: credentials.email,
      username: credentials.username,
      password: credentials.password
    }

    // run the register api
    const result = yield call(apis.register.loader, sendCredentials)
    const error = result.error
    const user = result.answer

    if(error) {
      yield put(actions.router.hook('authRegisterError', error))
      return
    }
    else if(!user || !user.Created) {
      yield put(actions.router.hook('authRegisterError', 'user was not created'))
      return
    }
    else {
      yield call(login, credentials)
      yield put(actions.router.hook('authRegisterSuccess', user))
      return user
    }
  }

  // called when the user clicks the submit button
  function* registerSubmit() {
    const credentials = yield call(formValuesIfValid, 'authRegister')
    if(credentials) {
      yield call(register, credentials)  
    }
  }

  function* logout() {
    deleteCachedCredentials()
    yield put(actions.value.set('user', null))
    yield put(actions.router.redirect('/dashboard'))
  }

  function* loginSuccess(user) {
    yield put(actions.router.redirect('/dashboard'))
  }

  function* registerSuccess(user) {
    yield put(actions.router.redirect('/dashboard'))
  }

  // control access to routes based on the user
  // if the route (or any parent route) has a 'user={true,false}'
  function* authenticateRoute() {
    const userSetting = yield select(state => selectors.router.firstValue(state, 'user'))
    const redirectSetting = yield select(state => selectors.router.firstValue(state, 'authRedirect'))
    // this route has no opinion about the user
    if(typeof(userSetting) != 'boolean') return true
    const user = yield select(state => selectors.valueSelector(state, 'user'))
    const hasUser = user ? true : false
    const isRouteAuthenticated = hasUser == userSetting
    if(!isRouteAuthenticated) {
      yield put(actions.router.redirect(redirectSetting || '/'))
      return false
    }
    return true
  }

  ///////////////////////////////////////
  ///////////////////////////////////////
  // utils

  function* formValuesIfValid(name) {
    const valid = yield select(selectors.form[name].valid)
    if(!valid) {
      yield put(actions.forms.touchAll(name))
      return
    }
    const credentials = yield select(selectors.form[name].values)
    return credentials
  }



  return {
    initialize,
    logout,
    status,
    login,
    loginSubmit,
    register,
    registerSubmit,
    loginSuccess,
    registerSuccess,
    authenticateRoute
  }
}

export default AuthSagas
