import processLoaders from 'template-ui/lib/plugins/api/processLoaders'

import * as selectors from '../selectors'
import JsonRpc from './jsonrpc'

// switch which type of auth driver we are using here
import auth from './auth/basic'
import repo from './repo'
import billing from './billing'
import config from './config'
import tools from '../tools'

// this tells the server to not reply with basic auth headers
// this prevents the browser opening a login window each time
const disableBasicAuthWindowParams = (payload, state) => ({
  disableBasicAuthWindow: 'y'
})

// grab the credentials from the redux storage and use them to make a request
const reducerCredentialsConnector = JsonRpc({
  // use the auth driver to inject the current credentials
  getHeaders: (payload, state) => {
    const reduxUserState = selectors.auth.user(state)
    return auth.getHeaders({
      Name: reduxUserState.Name,
      Password: reduxUserState.Password
    })
  },
  getParams: disableBasicAuthWindowParams
})

// grab the credentials from the payload
// (used in the login case where we don't want to reduce the credentials until login succeeded)
const payloadCredentialsConnector = JsonRpc({
  // use the auth driver to inject the current credentials
  getHeaders: (payload, state) => {
    return auth.getHeaders({
      Name: payload.Name,
      Password: payload.Password
    })
  },
  getParams: disableBasicAuthWindowParams
})

// wrap an pure api call (that returns an object)
// with the connector that returns a promise
// the connector is passed the state so it can inject auth credentials
const wrapper = (handler, connector = reducerCredentialsConnector) => (payload, state) => connector(handler(payload), state)

// these apis are processed and so each will have:
//
//  * name
//  * actions   - request,response + error actions for this api
//  * handler   - the raw promise generator
//  * loader    - the api saga loader (which triggers request, response + error actions)
//
// the following is a map of name -> handler
const loaders = {
  authLogin: {
    handler: wrapper(auth.login, payloadCredentialsConnector),
    // these are options passed to the apiSaga runner not the api
    options: {
      processError: (error) => {
        tools.devRun(() => {
          console.log('login error')
          console.dir(error)
        })
        return 'incorrect details'
      }
    }
  },
  // register is not wrapped - it does not need auth and works with ajax not rpc
  authRegister: auth.register,

  // all these methods need to be wrapped - they need auth
  repoList: wrapper(repo.list),
  repoCreate: wrapper(repo.create),
  repoLoadCommits: wrapper(repo.loadCommits),
  repoAddCollaborator: wrapper(repo.addCollaborator),

  billingSubmitPayment: wrapper(billing.submitPayment),

  configLoad: wrapper(config.load)
}

const processedLoaders = processLoaders(loaders)
const apis = processedLoaders.apis

export const actions = processedLoaders.actions
export default apis
