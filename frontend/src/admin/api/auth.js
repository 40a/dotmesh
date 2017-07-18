import config from '../config'
const url = (path) => config.api + path

export const status = (payload) => {
  return new Promise(resolve => {
    resolve({
      loggedIn: false
    })
  })
}

export const login = (payload) => {
  return new Promise(resolve => {
    resolve({
      registered: false,
      error: 'tbc'
    })
  })
}

export const register = (payload) => {
  return new Promise(resolve => {
    resolve({
      registered: false,
      error: 'tbc'
    })
  })
}

const userApi = {
  status,
  login,
  register
}

export default userApi