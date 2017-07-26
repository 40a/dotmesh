import { request } from 'template-ui/lib/utils/ajax'
import tools from './tools'

export const status = (payload, state) => {
  return request({
    url: tools.url('/auth/status')
  })
}

export const login = (payload, state) => {
  return request({
    method: 'post',
    url: tools.url('/auth/login'),
    data: payload
  })
}

export const register = (payload, state) => {
  return request({
    method: 'post',
    url: tools.url('/auth/register'),
    data: payload
  })
}

const userApi = {
  status,
  login,
  register
}

export default userApi