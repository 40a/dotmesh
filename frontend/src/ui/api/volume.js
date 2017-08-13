import tools from './tools'

export const list = (payload) => {
  return new Promise(resolve => {
    throw new Error('this is a test')
  })
}

const volumeApi = {
  list
}

export default volumeApi