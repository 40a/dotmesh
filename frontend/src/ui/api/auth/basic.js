import { request } from 'template-ui/lib/utils/ajax'
import tools from '../tools'

// login means "do these credentials allow us to see the Ping endpoint"
export const login = (payload) => {
  return tools.rpc({
    method: 'Ping',
    headers: getHeaders(payload.credentials),
    httpParams: {
      disableBasicAuth: 'y'
    }
  })
}

export const register = (payload) => {
  return new Promise(resolve => {
    throw new Error('this is a test')
  })
}

const encodeCredentials = (username, password) => new Buffer(username + ':' + password).toString('base64')

export const getHeaders = (credentials) => {
  return {
    Authorization: `Basic ${encodeCredentials(credentials.username, credentials.password)}`
  }
}

const userApi = {
  register,
  login,
  getHeaders
}

export default userApi