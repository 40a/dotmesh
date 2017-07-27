import { request } from 'template-ui/lib/utils/ajax'
import tools from '../tools'

export const status = (payload) => {
  return tools.rpc({
    method: 'Ping',
    headers: payload.headers
  })
}

export const register = (payload) => {
  return new Promise(resolve => {
    throw new Error('this is a test')
  })
}

const userApi = {
  status,
  register
}

export default userApi