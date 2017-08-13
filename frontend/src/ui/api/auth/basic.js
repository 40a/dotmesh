import { request } from 'template-ui/lib/utils/ajax'
import tools from '../tools'

// login means "do these credentials allow us to see the Ping endpoint"
export const login = (payload) => ({
  method: 'Ping'
})

export const register = (payload) => {
  return new Promise(resolve => {
    throw new Error('this is a test')
  })
}

export const encodeCredentials = (username, password) => new Buffer(username + ':' + password).toString('base64')

export const getHeaders = (credentials) => {
  console.log('-------------------------------------------');
  console.log('encide')
  console.dir(credentials)
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