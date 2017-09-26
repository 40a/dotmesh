import config from './config'

const logger = (st) => {
  if(process.env.NODE_ENV !== 'development') return
  console.log(st)
}

const devRun = (fn) => {
  if(process.env.NODE_ENV !== 'development') return
  fn()
}

const url = (path) => config.basepath + path

const tools = {
  logger,
  url,
  devRun
}

export default tools