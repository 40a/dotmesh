import { request } from 'template-ui/lib/utils/ajax'
import { userSelector } from '../selectors'
import config from '../config'

let requestCounter = 0
const rpc = (method, params) => {
  const id = requestCounter++
  return request({
    method: 'post',
    url: config.rpcUrl,
    data: {
      jsonrpc: '2.0',
      id,
      method,
      params
    }
  })
  .then(data => data.result)
}

const encodeBasicAuthDetails = (username, password) => 'Basic ' + new Buffer(username + ':' + password).toString('base64')
const authHeaders = (encodedDetails) => {
  return {
    Authorization: `Basic ${encodedDetails}`
  }
}

const tools = {
  rpc,
  encodeBasicAuthDetails,
  authHeaders
}

export default tools