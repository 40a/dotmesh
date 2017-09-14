import { take, put, call, fork, select, all, takeLatest, takeEvery, cancel } from 'redux-saga/effects'
import { delay } from 'redux-saga'

import config from '../config'
import forms from '../forms'
import * as actions from '../actions'
import * as selectors from '../selectors'

import tools from '../tools'

const REQUIRED_APIS = [
  'billingSubmitPayment'
]

const BillingSagas = (opts = {}) => {
  if(!opts.apis) throw new Error('auth saga requires a api option')
  const apis = opts.apis
  REQUIRED_APIS.forEach(name => {
    if(!apis[name]) throw new Error(`${name} api required`)
  })

  // the token is the object we get back from the frontend checkout.js
  // we extract the 'id' prop which is all the server needs
  function* tokenReceived(tokenObject) {

    const token = tokenObject.id
    const plan = config.devmodePlanName

    const payload = {
      token,
      plan
    }

    const { answer, error } = yield call(apis.billingSubmitPayment.loader, payload)

    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.log('answer')
    console.dir(answer)
    console.log('error')
    console.dir(error)
  }

  return {
    tokenReceived
  }
}

export default BillingSagas
