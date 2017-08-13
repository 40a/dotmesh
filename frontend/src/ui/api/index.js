import processLoaders from 'template-ui/lib/plugins/api/processLoaders'

import * as selectors from '../selectors'
import JsonRpc from './jsonrpc'

// switch which type of auth driver we are using here
import auth from './auth/basic'
import volume from './volume'

// a HTTP Basic auth version of the JSONRPC connector
// you can switch out this connector / use multiple connectors
// by creating multiple wrappers (see below)
const connector = JsonRpc({
  // use the auth driver to inject the current credentials
  getHeaders: (payload, state) => auth.getHeaders(selectors.auth.user(state)),
  // we don't want the browser popping up annoying auth windows
  // the backend golang server is setup for this param name
  getParams: (payload, state) => ({
    disableBasicAuthWindow: 'y'
  })
})

// wrap an pure api call (that returns an object)
// with the connector that returns a promise
// the connector is passed the state so it can inject auth credentials
const wrapper = (handler) => (payload, state) => connector(handler(payload), state)

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
    handler: wrapper(auth.login),
    // these are options passed to the apiSaga runner not the api
    options: {
      processError: (error) => 'incorrect details'
    }
  },
  // register is not wrapped - it does not need auth and works with ajax not rpc
  authRegister: auth.register,

  // all these methods need to be wrapped - they need auth
  volumeList: wrapper(volume.list)
}

const processedLoaders = processLoaders(loaders)
const apis = processedLoaders.apis

export const actions = processedLoaders.actions
export default apis