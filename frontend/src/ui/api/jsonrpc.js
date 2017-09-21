import { request } from 'template-ui/lib/utils/ajax'
import config from '../config'

const noop = () => ({})

//
// * getHeaders - a function that given the payload and state will return headers to inject to each request
// * getParams - a function that given the payload and state will return query params to inject to each request
//
//
//
// the returned executor is a function that accepts (payload, state):
//
//  * payload
//    * method
//    * params

const RPCExecutor = (opts = {}) => {
  let requestCounter = 0
  const getHeaders = opts.getHeaders || noop
  const getParams = opts.getParams || noop
  const rpc = (payload = {}, state) => {
    if(!payload.method) throw new Error('method needed')

    const id = requestCounter++
    const method = payload.method
    const params = payload.params || {}

    const httpHeaders = getHeaders(payload, state)
    const httpParams = getParams(payload, state)

    return request({
      method: 'post',
      url: config.rpcUrl,
      headers: httpHeaders,
      params: httpParams,
      data: {
        jsonrpc: '2.0',
        id,
        method: [config.rpcNamespace, method].join('.'),
        params
      }
    })
    .then(data => {
      if(data.id != id) throw new Error(`request id ${id} does not match response id ${data.id}`)
      return data.result
    })
  }
  return rpc
}


export default RPCExecutor