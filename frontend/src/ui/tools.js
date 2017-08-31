import config from './config'

const logger = (st) => {
  if(process.env.NODE_ENV !== 'development') return
  console.log(st)
}

const url = (path) => config.basepath + path

const tools = {
  logger,
  url
}

export default tools