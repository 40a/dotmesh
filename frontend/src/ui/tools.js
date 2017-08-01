const logger = (st) => {
  if(process.env.NODE_ENV !== 'development') return
  console.log(st)
}

const tools = {
  logger
}

export default tools