import { request } from 'template-ui/lib/utils/ajax'
import config from '../config'

let requestCounter = 0
const rpc = (opts = {}) => {
  if(!opts.method) throw new Error('method needed')

  // rpc
  const id = requestCounter++
  const method = opts.method
  const params = opts.params || {}
  
  // top level http request
  const httpParams = opts.httpParams || {}
  const headers = opts.headers || {}

  return request({
    method: 'post',
    url: config.rpcUrl,
    headers,
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
    if(!data.result) throw new Error(`no result found in response`)
    return data.result
  })
}

const tools = {
  rpc
}

export default tools